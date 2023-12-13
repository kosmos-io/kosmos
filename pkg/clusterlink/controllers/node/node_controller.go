package node

import (
	"context"
	"fmt"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	controllerName = "node-controller"
	RequeueTime    = 10 * time.Second
	clusterLabel   = "kosmos.io/cluster"
)

type Reconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ClusterLinkClient versioned.Interface
	ClusterName       string
}

var predicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		node := createEvent.Object.(*corev1.Node)
		return !utils.IsKosmosNode(node) && !utils.IsExcludeNode(node)
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		node := updateEvent.ObjectNew.(*corev1.Node)
		return !utils.IsKosmosNode(node) && !utils.IsExcludeNode(node)
	},
	DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
		node := deleteEvent.Object.(*corev1.Node)
		return !utils.IsKosmosNode(node) && !utils.IsExcludeNode(node)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("node controller starts to reconcile cluster %s", request.NamespacedName.Name)

	clusterNodeName := ClusterNodeName(r.ClusterName, request.Name)
	var node corev1.Node
	if err := r.Get(ctx, request.NamespacedName, &node); err != nil {
		if apierrors.IsNotFound(err) {
			err := r.ClusterLinkClient.KosmosV1alpha1().ClusterNodes().Delete(ctx, clusterNodeName, metav1.DeleteOptions{})
			if err != nil {
				klog.Warningf("delete cluster node %s err: %v", clusterNodeName, err)
			}
			return reconcile.Result{}, nil
		}
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}

	if !node.GetDeletionTimestamp().IsZero() {
		err := r.ClusterLinkClient.KosmosV1alpha1().ClusterNodes().Delete(ctx, clusterNodeName, metav1.DeleteOptions{})
		if err != nil {
			klog.Warningf("delete cluster node %s err: %v", node.Name, err)
		}
		return reconcile.Result{}, nil
	}

	// add or update clusternode
	var internalIP string
	var internalIP6 string
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			_, proto := ParseIP(address.Address)
			if proto == 4 && len(internalIP) == 0 {
				internalIP = address.Address
			}
			if proto == 6 && len(internalIP6) == 0 {
				internalIP6 = address.Address
			}
			if len(internalIP) > 0 && len(internalIP6) > 0 {
				break
			}
		}
	}
	clusterNode := &clusterlinkv1alpha1.ClusterNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterNodeName,
			Labels: map[string]string{
				clusterLabel: r.ClusterName,
			},
		},
	}

	err := CreateOrUpdateClusterNode(r.ClusterLinkClient, clusterNode, func(n *clusterlinkv1alpha1.ClusterNode) error {
		n.Spec.NodeName = node.Name
		n.Spec.ClusterName = r.ClusterName
		// n.Spec.InterfaceName while set by clusterlink-agent
		return nil
	})
	if err != nil {
		klog.Errorf("can nod update or create clusterNode %s err: %v", clusterNode.Name, err)
		return reconcile.Result{Requeue: true}, nil
	}
	return reconcile.Result{}, nil
}

type MutateClusterNodeFn func(node *clusterlinkv1alpha1.ClusterNode) error

func CreateOrUpdateClusterNode(client versioned.Interface, node *clusterlinkv1alpha1.ClusterNode, f MutateClusterNodeFn) error {
	clusterNode, err := client.KosmosV1alpha1().ClusterNodes().Get(context.Background(), node.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := f(node); err != nil {
			return err
		}
		_, err := client.KosmosV1alpha1().ClusterNodes().Create(context.Background(), node, metav1.CreateOptions{})
		if err != nil {
			return err
		} else {
			return nil
		}
	}
	if err := f(clusterNode); err != nil {
		return err
	}
	_, err = client.KosmosV1alpha1().ClusterNodes().Update(context.Background(), clusterNode, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) CleanOrphan() error {
	k8sNodeList := &corev1.NodeList{}
	if err := r.List(context.Background(), k8sNodeList); err != nil {
		return err
	}
	k8sNodeNameSet := make(map[string]struct{})
	for _, node := range k8sNodeList.Items {
		k8sNodeNameSet[node.Name] = struct{}{}
	}

	clusterNodes, err := r.ClusterLinkClient.KosmosV1alpha1().ClusterNodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", clusterLabel, r.ClusterName),
	})
	if err != nil {
		return err
	}
	var errs []error
	cnt := 0
	for i := range clusterNodes.Items {
		clusterNode := &clusterNodes.Items[i]
		if _, ok := k8sNodeNameSet[clusterNode.Spec.NodeName]; !ok {
			if err := r.ClusterLinkClient.KosmosV1alpha1().ClusterNodes().Delete(context.Background(), clusterNode.Name, metav1.DeleteOptions{}); err != nil {
				errs = append(errs, err)
				klog.Warningf("failed to delete clusterNode %s", clusterNode.Name)
			} else {
				cnt++
			}
		}
	}
	if len(errs) != 0 {
		return errors.NewAggregate(errs)
	}
	if cnt > 0 {
		klog.Infof("successfully deleted %d orphan clusterNodes", cnt)
	}

	return nil
}

func (r *Reconciler) SetupWithManager(mgr manager.Manager, stopChan <-chan struct{}) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				err := r.CleanOrphan()
				if err != nil {
					klog.Warningf("clear orphan err: %v", err)
				}
			case <-stopChan:
				ticker.Stop()
				return
			}
		}
	}()
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Node{}, builder.WithPredicates(predicatesFunc)).
		Complete(r)
}

func (r *Reconciler) CleanResource() error {
	list, err := r.ClusterLinkClient.KosmosV1alpha1().ClusterNodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kosmos.io/cluster=%s", r.ClusterName),
	})
	if err != nil {
		klog.Errorf("list clusterNode err: %v", err)
		return err
	}
	var errs []error
	for i := range list.Items {
		node := &list.Items[i]
		err := r.ClusterLinkClient.KosmosV1alpha1().ClusterNodes().Delete(context.TODO(), node.GetName(), metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("node controller failed delete clustersNodes: %v", errors.NewAggregate(errs))
	}
	return nil
}

func ClusterNodeName(clusterName, nodeName string) string {
	return clusterName + "-" + nodeName
}

func ParseIP(s string) (net.IP, int) {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, 0
	}
	ipv4 := ip.To4()
	if ipv4 != nil {
		return ipv4, 4
	}
	return ip, 6
}
