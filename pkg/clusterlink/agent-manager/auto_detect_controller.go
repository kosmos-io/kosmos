package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/controllers/node"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network"
	"github.com/kosmos.io/kosmos/pkg/utils"
	interfacepolicy "github.com/kosmos.io/kosmos/pkg/utils/interface-policy"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted/autodetection"
)

const (
	AutoDetectControllerName = "cluster-node-controller"
	AutoDetectRequeueTime    = 10 * time.Second
)

const (
	// nolint:revive
	AUTODETECTION_METHOD_CAN_REACH = "can-reach="
)

type AutoDetectReconciler struct {
	client.Client
	ClusterName string
	NodeName    string
}

func (r *AutoDetectReconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	skipEvent := func(obj client.Object) bool {
		eventObj, ok := obj.(*kosmosv1alpha1.ClusterNode)
		if !ok {
			return false
		}

		if eventObj.Name != node.ClusterNodeName(r.ClusterName, r.NodeName) {
			klog.V(4).Infof("skip event, reconcile node name: %s, current node name: %s-%s", eventObj.Name, r.ClusterName, r.NodeName)
			return false
		}

		return true
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(AutoDetectControllerName).
		WithOptions(controller.Options{}).
		For(&kosmosv1alpha1.ClusterNode{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return skipEvent(createEvent.Object)
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return skipEvent(updateEvent.ObjectNew)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return skipEvent(genericEvent.Object)
			},
		})).
		Watches(&source.Kind{Type: &kosmosv1alpha1.Cluster{}}, handler.EnqueueRequestsFromMapFunc(r.newClusterMapFunc())).
		Complete(r)
}

func (r *AutoDetectReconciler) newClusterMapFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		var requests []reconcile.Request
		cluster := a.(*kosmosv1alpha1.Cluster)
		klog.V(4).Infof("auto detect cluster change: %s, currentNode cluster name: %s", cluster.Name, r.ClusterName)
		if cluster.Name == r.ClusterName {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Name: node.ClusterNodeName(r.ClusterName, r.NodeName),
			}})
		}
		return requests
	}
}

// nolint:revive
func (r *AutoDetectReconciler) detectInterfaceName(ctx context.Context) (string, error) {
	var Cluster kosmosv1alpha1.Cluster

	if err := r.Get(ctx, types.NamespacedName{
		Name:      r.ClusterName,
		Namespace: "",
	}, &Cluster); err != nil {
		return "", err
	}

	if Cluster.Spec.ClusterLinkOptions != nil {
		defaultNICName := interfacepolicy.GetInterfaceName(Cluster.Spec.ClusterLinkOptions.NICNodeNames, r.NodeName, Cluster.Spec.ClusterLinkOptions.DefaultNICName)

		if defaultNICName != network.AutoSelectInterfaceFlag {
			return defaultNICName, nil
		}

		method := Cluster.Spec.ClusterLinkOptions.AutodetectionMethod
		// TODO: set default reachable ip when defaultNICName == * and meth == ""
		if method == "" {
			method = fmt.Sprintf("%s%s", AUTODETECTION_METHOD_CAN_REACH, "8.8.8.8")
		}
		if strings.HasPrefix(method, AUTODETECTION_METHOD_CAN_REACH) {
			// Autodetect the IP by connecting a UDP socket to a supplied address.
			destStr := strings.TrimPrefix(method, AUTODETECTION_METHOD_CAN_REACH)

			version := 4
			if utils.IsIPv6(destStr) {
				version = 6
			}

			if i, _, err := autodetection.ReachDestination(destStr, version); err != nil {
				return "", err
			} else {
				return i.Name, nil
			}
		}
	}
	return "", fmt.Errorf("can not detect nic")
}

func detectIP(interfaceName string) (string, string) {
	detectFunc := func(version int) (string, error) {
		_, n, err := autodetection.FilteredEnumeration([]string{interfaceName}, nil, nil, version)
		if err != nil {
			return "", fmt.Errorf("auto detect interface error: %v, version: %d", err, version)
		}

		if len(n.IP) == 0 {
			return "", fmt.Errorf("auto detect interface error: ip is nil, version: %d", version)
		}
		return n.IP.String(), nil
	}

	ipv4, err := detectFunc(4)
	if err != nil {
		klog.Warning(err)
	}
	ipv6, err := detectFunc(6)
	if err != nil {
		klog.Warning(err)
	}
	return ipv4, ipv6
}

func shouldUpdate(old, new kosmosv1alpha1.ClusterNode) bool {
	return old.Spec.IP != new.Spec.IP ||
		old.Spec.IP6 != new.Spec.IP6 ||
		old.Spec.InterfaceName != new.Spec.InterfaceName
}

func (r *AutoDetectReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("########################### auto_detect_controller starts to reconcile %s ###########################", request.NamespacedName)
	defer klog.V(4).Infof("####################### auto_detect_controller finished to reconcile %s ###########################", request.NamespacedName)

	// get clusternode
	var clusterNode kosmosv1alpha1.ClusterNode
	if err := r.Get(ctx, request.NamespacedName, &clusterNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("auto_detect_controller cluster node not found %s", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		klog.Errorf("get clusternode %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: AutoDetectRequeueTime}, nil
	}

	// skip when deleting
	if !clusterNode.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	// update clusterNode
	newClusterNode := clusterNode.DeepCopy()

	currentInterfaceName, err := r.detectInterfaceName(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("cluster is not found, %s", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		klog.Errorf("get cluster %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: AutoDetectRequeueTime}, nil
	}

	// update interface
	newClusterNode.Spec.InterfaceName = currentInterfaceName

	klog.V(4).Infof("auto detect interface name: %s", currentInterfaceName)

	// detect IP by Name
	ipv4, ipv6 := detectIP(currentInterfaceName)
	klog.V(4).Infof("auto detect ipv4: %s, ipv6: %s", ipv4, ipv6)

	if ipv4 != "" {
		newClusterNode.Spec.IP = ipv4
	}
	if ipv6 != "" {
		newClusterNode.Spec.IP6 = ipv6
	}

	if shouldUpdate(*newClusterNode, clusterNode) {
		if err := r.Update(ctx, newClusterNode); err != nil {
			klog.Errorf("update clusternode %s error: %v", request.NamespacedName, err)
			return reconcile.Result{RequeueAfter: AutoDetectRequeueTime}, nil
		}
		klog.V(4).Infof("update clusternode interface: %s, ipv4: %s, ipv6:%s, successed!", newClusterNode.Spec.InterfaceName, newClusterNode.Spec.IP, newClusterNode.Spec.IP6)
	} else {
		klog.V(4).Info("clusternode is not need to update")
	}

	return reconcile.Result{}, nil
}
