package controller

import (
	"context"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

type VirtualClusterInitController struct {
	client.Client
	Config        *rest.Config
	EventRecorder record.EventRecorder
}

func (c *VirtualClusterInitController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing virtual cluster", "virtual cluster", request, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing virtual cluster", "virtual cluster", request, "duration", time.Since(startTime))
	}()

	virtualCluster := &v1alpha1.VirtualCluster{}
	if err := c.Get(ctx, request.NamespacedName, virtualCluster); err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).InfoS("Virtual Cluster has been deleted", "Virtual Cluster", request)
			return reconcile.Result{}, nil

		}
		return reconcile.Result{}, nil
	}

	if !virtualCluster.DeletionTimestamp.IsZero() {
		//TODO The object is being deleted
	}
	//TODO The object is being updated

	if virtualCluster.Status.Phase == constants.VirtualClusterStatusCompleted {
		klog.Infof("cluster's status is %s, skip", virtualCluster.Status.Phase)
		return reconcile.Result{}, nil
	}

	err := c.syncVirtualCluster(virtualCluster)
	if err != nil {
		return reconcile.Result{}, nil
	}
	return reconcile.Result{}, nil
}

func (c *VirtualClusterInitController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(constants.InitControllerName).
		WithOptions(controller.Options{}).
		For(&v1alpha1.VirtualCluster{},
			builder.WithPredicates(predicate.Funcs{
				//	UpdateFunc: c.onVirtualClusterUpdate,
				CreateFunc: func(createEvent event.CreateEvent) bool {
					return true
				},
			})).
		Complete(c)
}

func (c *VirtualClusterInitController) syncVirtualCluster(virtualCluster *v1alpha1.VirtualCluster) error {
	klog.V(2).Infof("Reconciling virtual cluster", "name", virtualCluster.Name)
	executer, err := NewExecuter(virtualCluster, c.Client, c.Config)
	if err != nil {
		return err
	}
	return executer.Execute()
}

// The object is not being deleted, so if it does not have our finalizer,
// then lets add the finalizer and update the object. This is equivalent
// registering our finalizer.
func (c *VirtualClusterInitController) ensureVirtualCluster(ctx context.Context, virtualCluster *v1alpha1.VirtualCluster) error {
	updated := controllerutil.AddFinalizer(virtualCluster, constants.ControllerFinalizerName)
	if _, isExist := virtualCluster.Labels[constants.DisableCascadingDeletionLabel]; !isExist {
		labelMap := labels.Merge(virtualCluster.GetLabels(), labels.Set{constants.DisableCascadingDeletionLabel: "false"})
		virtualCluster.SetLabels(labelMap)
		updated = true
	}
	older := virtualCluster.DeepCopy()
	if updated || !reflect.DeepEqual(virtualCluster.Spec, older.Spec) {
		return c.Update(ctx, virtualCluster)
	}

	return nil
}
