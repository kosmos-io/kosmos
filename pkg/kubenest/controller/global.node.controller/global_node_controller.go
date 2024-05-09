package globalnodecontroller

import (
	"context"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type GlobalNodeController struct {
	client.Client
	RootClientSet kubernetes.Interface
	EventRecorder record.EventRecorder
	KosmosClient  versioned.Interface
}

func (r *GlobalNodeController) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.GlobalNodeControllerName).
		WithOptions(controller.Options{}).
		For(&v1alpha1.GlobalNode{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return true
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return true
			},
		})).
		Complete(r)
}

func (r *GlobalNodeController) SyncState(ctx context.Context, globalNode *v1alpha1.GlobalNode) error {
	if globalNode.Spec.State == v1alpha1.NodeInUse {
		klog.V(4).Infof("global-node-controller: SyncState: node is in use %s, skip", globalNode.Name)
		return nil
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var hostNode v1.Node
		if err := r.Get(ctx, types.NamespacedName{Name: globalNode.Name}, &hostNode); err != nil {
			klog.Errorf("global-node-controller: SyncState: can not get global node, err: %s", globalNode.Name)
			return err
		}

		updateHostNode := hostNode.DeepCopy()

		v, ok := updateHostNode.Labels[constants.StateLabelKey]
		if ok && v == string(globalNode.Spec.State) {
			return nil
		}

		updateHostNode.Labels[constants.StateLabelKey] = string(globalNode.Spec.State)
		if err := r.Update(ctx, updateHostNode); err != nil {
			klog.Errorf("global-node-controller: SyncState: update node label failed, err: %s", globalNode.Name)
			return err
		}
		return nil
	})
	return err
}

func (r *GlobalNodeController) SyncLabel(ctx context.Context, globalNode *v1alpha1.GlobalNode) error {
	var client kubernetes.Interface
	if globalNode.Spec.State != v1alpha1.NodeInUse {
		client = r.RootClientSet
	} else {
		vclist, err := r.KosmosClient.KosmosV1alpha1().VirtualClusters("").List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("global-node-controller: SyncState: cannot list virtual cluster, err: %s", globalNode.Name)
			return err
		}
		var targetVirtualCluster v1alpha1.VirtualCluster
		for _, v := range vclist.Items {
			if v.Name == globalNode.Status.VirtualCluster {
				targetVirtualCluster = v
				break
			}
		}
		virtualClient, err := util.GenerateKubeclient(&targetVirtualCluster)
		if err != nil {
			klog.Errorf("global-node-controller: SyncState: cannot generate kubeclient, err: %s", globalNode.Name)
			return err
		}

		client = virtualClient
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		hostNode, err := client.CoreV1().Nodes().Get(ctx, globalNode.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("global-node-controller: SyncState: can not get global node, err: %s", globalNode.Name)
			return err
		}

		updateHostNode := hostNode.DeepCopy()
		needUpdate := false
		for k, v := range globalNode.Spec.Labels {
			if updateHostNode.Labels[k] != v {
				updateHostNode.Labels[k] = v
				needUpdate = true
			}
		}
		if !needUpdate {
			return nil
		}

		if _, err := client.CoreV1().Nodes().Update(ctx, updateHostNode, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("global-node-controller: SyncState: update node label failed, err: %s", globalNode.Name)
			return err
		}
		return nil
	})
	return err
}

func (r *GlobalNodeController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ global-node-controller start to reconcile %s ============", request.NamespacedName)
	defer klog.V(4).Infof("============ global-node-controller finish to reconcile %s ============", request.NamespacedName)

	var globalNode v1alpha1.GlobalNode
	if err := r.Get(ctx, request.NamespacedName, &globalNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("global-node-controller: can not found %s", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		klog.Errorf("get global-node %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	if err := r.SyncState(ctx, &globalNode); err != nil {
		klog.Errorf("sync State %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	} else {
		klog.V(4).Infof("sync state successed, %s", request.NamespacedName)
	}

	if err := r.SyncLabel(ctx, &globalNode); err != nil {
		klog.Errorf("sync label %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	} else {
		klog.V(4).Infof("sync label successed, %s", request.NamespacedName)
	}

	return reconcile.Result{}, nil
}
