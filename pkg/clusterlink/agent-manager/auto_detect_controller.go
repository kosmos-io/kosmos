package agent

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/agent-manager/autodetection"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network"
)

const (
	AutoDetectControllerName = "cluster-node-controller"
	AutoDetectRequeueTime    = 10 * time.Second
)

type AutoDetectReconciler struct {
	client.Client
	ClusterName string
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

		if eventObj.Spec.ClusterName != r.ClusterName {
			return false
		}

		if eventObj.Spec.InterfaceName == network.AutoSelectInterfaceFlag || len(eventObj.Spec.InterfaceName) == 0 {
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
		Complete(r)
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
	return old.Spec.IP != new.Spec.IP || old.Spec.IP6 != new.Spec.IP6
}

func (r *AutoDetectReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("########################### auto_detect_controller starts to reconcile %s ###########################", request.NamespacedName)
	defer klog.V(4).Infof("####################### auto_detect_controller finished to reconcile %s ###########################", request.NamespacedName)

	// get clusternode
	var clusterNode kosmosv1alpha1.ClusterNode
	if err := r.Get(ctx, request.NamespacedName, &clusterNode); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		klog.Errorf("get clusternode %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: AutoDetectRequeueTime}, nil
	}

	// skip when deleting
	if !clusterNode.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	// only do autodetect when clusterNode.Spec.InterfaceName != * and not nil
	if clusterNode.Spec.InterfaceName == network.AutoSelectInterfaceFlag || len(clusterNode.Spec.InterfaceName) == 0 {
		return reconcile.Result{}, nil
	}

	// detect IP by Name
	ipv4, ipv6 := detectIP(clusterNode.Spec.InterfaceName)
	klog.V(4).Infof("auto detect ipv4: %s, ipv6: %s", ipv4, ipv6)

	// update clusterNode
	newClusterNode := clusterNode.DeepCopy()
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
		} else {
			klog.V(4).Infof("update clusternode interface: %s, ipv4: %s, ipv6:%s, successed!", newClusterNode.Spec.InterfaceName, newClusterNode.Spec.IP, newClusterNode.Spec.IP6)
		}
	} else {
		klog.V(4).Info("clusternode is not need to update")
	}

	return reconcile.Result{}, nil
}
