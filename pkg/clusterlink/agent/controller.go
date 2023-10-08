package agent

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	networkmanager "github.com/kosmos.io/kosmos/pkg/clusterlink/agent/network-manager"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network"
	kosmosv1alpha1lister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
)

const (
	controllerName = "cluster-node-controller"
	RequeueTime    = 30 * time.Second
)

type Reconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	NodeName         string
	ClusterName      string
	NetworkManager   *networkmanager.NetworkManager
	NodeConfigLister kosmosv1alpha1lister.NodeConfigLister
	ClusterLister    kosmosv1alpha1lister.ClusterLister
	DebounceFunc     func(f func())
}

func NetworkManager() *networkmanager.NetworkManager {
	net := network.NewNetWork()
	return networkmanager.NewNetworkManager(net)
}

var predicatesFunc = predicate.Funcs{
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
}

func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{}).
		For(&kosmosv1alpha1.NodeConfig{}, builder.WithPredicates(predicatesFunc)).
		Complete(r)
}

func (r *Reconciler) logResult(nodeConfigSyncStatus networkmanager.NodeConfigSyncStatus) {
	if nodeConfigSyncStatus == networkmanager.NodeConfigSyncException {
		klog.Warning("sync from crd failed!")
		klog.Warning(r.NetworkManager.GetReason())
	}
	if nodeConfigSyncStatus == networkmanager.NodeConfigSyncSuccess {
		klog.Info("sync from crd successfully!")
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("============ agent starts to reconcile %s ============", request.NamespacedName)

	var reconcileNode kosmosv1alpha1.NodeConfig
	if err := r.Get(ctx, request.NamespacedName, &reconcileNode); err != nil {
		// The resource no longer exists
		if apierrors.IsNotFound(err) {
			nodeConfigSyncStatus := r.NetworkManager.UpdateFromCRD(&kosmosv1alpha1.NodeConfig{
				Spec: kosmosv1alpha1.NodeConfigSpec{}})
			r.logResult(nodeConfigSyncStatus)
			return reconcile.Result{}, nil
		}
		klog.Errorf("get clusternode %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}

	klog.Infof("reconcile node name: %s, current node name: %s-%s", reconcileNode.Name, r.ClusterName, r.NodeName)
	if reconcileNode.Name != fmt.Sprintf("%s-%s", r.ClusterName, r.NodeName) {
		klog.Infof("not match, drop this event.")
		return reconcile.Result{}, nil
	}

	localCluster, err := r.ClusterLister.Get(r.ClusterName)
	if err != nil {
		klog.Errorf("could not get local cluster, clusterNode: %s, err: %v", r.NodeName, err)
		return reconcile.Result{}, nil
	}

	r.NetworkManager.UpdateConfig(localCluster)

	r.DebounceFunc(func() {
		nodeConfigSyncStatus := r.NetworkManager.UpdateFromCRD(&reconcileNode)
		r.logResult(nodeConfigSyncStatus)
	})

	return reconcile.Result{}, nil
}

func (r *Reconciler) StartTimer(ctx context.Context) {
	timer := time.NewTimer(RequeueTime)
	for {
		select {
		case <-timer.C:
			klog.Info("###################### start check ######################")
			nodeConfigSyncStatus := r.NetworkManager.UpdateFromChecker()
			r.logResult(nodeConfigSyncStatus)
			timer.Reset(RequeueTime)
		case <-ctx.Done():
			klog.Infoln("kill the timer")
			return
		}
	}
}
