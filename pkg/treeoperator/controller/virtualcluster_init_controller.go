package controller

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/constants"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/workflow"
)

type VirtualClusterInitController struct {
	client.Client
	Config        *rest.Config
	EventRecorder record.EventRecorder
}

var predicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		return true
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		newObj := updateEvent.ObjectNew.(*v1alpha1.VirtualCluster)
		oldObj := updateEvent.ObjectOld.(*v1alpha1.VirtualCluster)

		if !newObj.DeletionTimestamp.IsZero() {
			return true
		}

		return !reflect.DeepEqual(newObj.Spec, oldObj.Spec)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *VirtualClusterInitController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(constants.InitControllerName).
		WithOptions(controller.Options{}).
		For(&v1alpha1.VirtualCluster{}, builder.WithPredicates(predicatesFunc)).
		Complete(c)
}

func (c *VirtualClusterInitController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", constants.InitControllerName, request.Name)

	virtualCluster := &v1alpha1.VirtualCluster{}
	if err := c.Get(ctx, request.NamespacedName, virtualCluster); err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).InfoS("Virtual Cluster has been deleted", "Virtual Cluster", request)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	virtualClusterCopy := virtualCluster.DeepCopy()
	return controllerruntime.Result{}, c.syncVirtualCluster(virtualClusterCopy)
}

func (c *VirtualClusterInitController) syncVirtualCluster(virtualCluster *v1alpha1.VirtualCluster) error {
	klog.V(2).Infof("Reconciling Virtual Cluster", "name", virtualCluster.Name)
	executer, err := workflow.NewExecuter(virtualCluster, c.Client, c.Config)
	if err != nil {
		return err
	}
	return executer.Execute()
}
