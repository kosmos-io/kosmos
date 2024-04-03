package controller

import (
	"context"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
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
	"github.com/kosmos.io/kosmos/pkg/treeoperator/scheme"
)

const (
	InitControllerName            = "virtual-cluster-init-controller"
	DisableCascadingDeletionLabel = "operator.virtualcluster.io/disable-cascading-deletion"
	ControllerFinalizerName       = "operator.virtualcluster.io/finalizer"
)

type VirtualClusterInitController struct {
	client.Client
	Config        *rest.Config
	EventRecorder record.EventRecorder
}

func (c *VirtualClusterInitController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var wg sync.WaitGroup
	wg.Add(1) // 增加计数

	go func() error {
		defer wg.Done() // 在 goroutine 结束时减少计数

		startTime := time.Now()
		klog.V(4).InfoS("Started syncing virtual cluster", "virtual cluster", request, "startTime", startTime)
		defer func() {
			klog.V(4).InfoS("Finished syncing virtual cluster", "virtual cluster", request, "duration", time.Since(startTime))
		}()

		virtualCluster := &v1alpha1.VirtualCluster{}
		if err := c.Get(ctx, request.NamespacedName, virtualCluster); err != nil {
			if errors.IsNotFound(err) {
				klog.V(2).InfoS("Virtual Cluster has been deleted", "Virtual Cluster", request)
				return nil
			}
			return err
		}

		// The object is being deleted
		/*	if !virtualCluster.DeletionTimestamp.IsZero() {
				val, ok := virtualCluster.Labels[DisableCascadingDeletionLabel]
				if !ok || val == strconv.FormatBool(false) {
					if err := c.syncVirtualCluster(virtualCluster); err != nil {
						return controllerruntime.Result{}, err
					}
				}

				return c.removeFinalizer(ctx, virtualCluster)
			}

			if err := c.ensureVirtualCluster(ctx, virtualCluster); err != nil {
				return controllerruntime.Result{}, err
			}*/

		err := c.syncVirtualCluster(virtualCluster)
		if err != nil {
			return err
		}
		return nil
	}()
	wg.Wait() // 等待所有goroutine执行完成
	return controllerruntime.Result{}, nil
}

func (c *VirtualClusterInitController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(InitControllerName).
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

func (c *VirtualClusterInitController) ensureVirtualCluster(ctx context.Context, virtualCluster *v1alpha1.VirtualCluster) error {
	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	updated := controllerutil.AddFinalizer(virtualCluster, ControllerFinalizerName)
	if _, isExist := virtualCluster.Labels[DisableCascadingDeletionLabel]; !isExist {
		labelMap := labels.Merge(virtualCluster.GetLabels(), labels.Set{DisableCascadingDeletionLabel: "false"})
		virtualCluster.SetLabels(labelMap)
		updated = true
	}

	older := virtualCluster.DeepCopy()

	// Set the defaults for virtualCluster
	scheme.Scheme.Default(virtualCluster)

	if updated || !reflect.DeepEqual(virtualCluster.Spec, older.Spec) {
		return c.Update(ctx, virtualCluster)
	}

	return nil
}

func (c *VirtualClusterInitController) syncVirtualCluster(virtualCluster *v1alpha1.VirtualCluster) error {
	klog.V(2).Infof("Reconciling virtual cluster", "name", virtualCluster.Name)
	executer, err := NewExecuter(virtualCluster, c.Client, c.Config)
	if err != nil {
		return err
	}
	return executer.Execute()
}

func (c *VirtualClusterInitController) onVirtualClusterUpdate(updateEvent event.UpdateEvent) bool {
	newObj := updateEvent.ObjectNew.(*v1alpha1.VirtualCluster)
	oldObj := updateEvent.ObjectOld.(*v1alpha1.VirtualCluster)

	if !newObj.DeletionTimestamp.IsZero() {
		return true
	}

	return !reflect.DeepEqual(newObj.Spec, oldObj.Spec)
}

func (c *VirtualClusterInitController) removeFinalizer(ctx context.Context, virtualCluster *v1alpha1.VirtualCluster) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		newer := &v1alpha1.VirtualCluster{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(virtualCluster), newer); err != nil {
			return err
		}

		if controllerutil.RemoveFinalizer(newer, ControllerFinalizerName) {
			return c.Update(ctx, newer)
		}

		return nil
	})
}
