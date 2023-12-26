package mcs

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
	"github.com/kosmos.io/kosmos/pkg/utils/helper"
)

const ServiceExportControllerName = "service-export-controller"

// ServiceExportController watches serviceExport in root cluster and annotated the endpointSlice
type ServiceExportController struct {
	RootClient    client.Client
	EventRecorder record.EventRecorder
	Logger        logr.Logger
	// ReservedNamespaces are the protected namespaces to prevent Kosmos for deleting system resources
	ReservedNamespaces []string
	RateLimiterOptions flags.Options
	BackoffOptions     flags.BackoffOptions
}

func (c *ServiceExportController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", ServiceExportControllerName, request.NamespacedName.String())
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled =============", request.NamespacedName.String())
	}()

	serviceExport := &mcsv1alpha1.ServiceExport{}
	if err := c.RootClient.Get(ctx, request.NamespacedName, serviceExport); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		klog.Errorf("Get serviceExport (%s/%s)'s failed, Error: %v", serviceExport.Namespace, serviceExport.Name, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	// The serviceExport is being deleted, in which case we should clear endpointSlice.
	if !serviceExport.DeletionTimestamp.IsZero() {
		if err := c.removeAnnotation(request.Namespace, request.Name); err != nil {
			klog.Errorf("Remove serviceExport (%s/%s)'s annotation failed, Error: %v", serviceExport.Namespace, serviceExport.Name, err)
			return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
		}
		return c.removeFinalizer(serviceExport)
	}

	err := c.syncServiceExport(serviceExport)
	if err != nil {
		klog.Errorf("Sync serviceExport (%s/%s) failed, Error: %v", serviceExport.Namespace, serviceExport.Name, err)
		return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}
	return c.ensureFinalizer(serviceExport)
}

func (c *ServiceExportController) SetupWithManager(mgr manager.Manager) error {
	endpointSliceServiceExportFn := handler.MapFunc(
		func(object client.Object) []reconcile.Request {
			serviceName := helper.GetLabelOrAnnotationValue(object.GetLabels(), utils.ServiceKey)
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: object.GetNamespace(),
						Name:      serviceName,
					},
				},
			}
		},
	)

	endpointSlicePredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return c.shouldEnqueue(event.Object)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return c.shouldEnqueue(deleteEvent.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return c.shouldEnqueue(updateEvent.ObjectNew)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	},
	)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&mcsv1alpha1.ServiceExport{}).
		Watches(&source.Kind{Type: &discoveryv1.EndpointSlice{}},
			handler.EnqueueRequestsFromMapFunc(endpointSliceServiceExportFn),
			endpointSlicePredicate,
		).
		WithOptions(controller.Options{
			RateLimiter:             flags.DefaultControllerRateLimiter(c.RateLimiterOptions),
			MaxConcurrentReconciles: 2,
		}).
		Complete(c)
}

func (c *ServiceExportController) shouldEnqueue(object client.Object) bool {
	eps, ok := object.(*discoveryv1.EndpointSlice)
	if !ok {
		return false
	}

	if slices.Contains(c.ReservedNamespaces, eps.Namespace) {
		return false
	}

	return true
}

func (c *ServiceExportController) removeAnnotation(namespace, name string) error {
	var err error
	selector := labels.SelectorFromSet(
		map[string]string{
			utils.ServiceKey: name,
		},
	)
	epsList := &discoveryv1.EndpointSliceList{}
	err = c.RootClient.List(context.TODO(), epsList, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
	})

	if err != nil {
		klog.Errorf("List endpointSlice in %s failed, Error: %v", namespace, err)
		return err
	}

	endpointSlices := epsList.Items
	for i := range endpointSlices {
		newEps := &endpointSlices[i]
		if newEps.DeletionTimestamp != nil {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting and does not need to remove serviceExport annotation", namespace, newEps.Name)
			continue
		}

		err = c.updateEndpointSlice(newEps, c.RootClient, func(eps *discoveryv1.EndpointSlice) {
			helper.RemoveAnnotation(eps, utils.ServiceExportLabelKey)
		})
		if err != nil {
			klog.Errorf("Update endpointSlice (%s/%s) failed, Error: %v", namespace, newEps.Name, err)
			return err
		}
	}
	return nil
}

// nolint:dupl
func (c *ServiceExportController) updateEndpointSlice(eps *discoveryv1.EndpointSlice, rootClient client.Client, modifyEps func(eps *discoveryv1.EndpointSlice)) error {
	return retry.RetryOnConflict(flags.DefaultUpdateRetryBackoff(c.BackoffOptions), func() error {
		modifyEps(eps)
		updateErr := rootClient.Update(context.TODO(), eps)
		if apierrors.IsNotFound(updateErr) {
			return nil
		}
		if updateErr == nil {
			return nil
		}
		klog.Errorf("Failed to update endpointSlice %s/%s: %v", eps.Namespace, eps.Name, updateErr)
		newEps := &discoveryv1.EndpointSlice{}
		getErr := rootClient.Get(context.TODO(), client.ObjectKey{Namespace: eps.Namespace, Name: eps.Name}, newEps)
		if getErr == nil {
			//Make a copy, so we don't mutate the shared cache
			eps = newEps.DeepCopy()
		} else {
			if apierrors.IsNotFound(getErr) {
				return nil
			} else {
				klog.Errorf("Failed to get updated endpointSlice %s/%s: %v", eps.Namespace, eps.Name, getErr)
			}
		}

		return updateErr
	})
}

// nolint:dupl
func (c *ServiceExportController) updateServiceExport(export *mcsv1alpha1.ServiceExport, rootClient client.Client, modifyExport func(export *mcsv1alpha1.ServiceExport)) error {
	return retry.RetryOnConflict(flags.DefaultUpdateRetryBackoff(c.BackoffOptions), func() error {
		modifyExport(export)
		updateErr := rootClient.Update(context.TODO(), export)
		if apierrors.IsNotFound(updateErr) {
			return nil
		}
		if updateErr == nil {
			return nil
		}
		klog.Errorf("Failed to update serviceExport %s/%s: %v", export.Namespace, export.Name, updateErr)
		newExport := &mcsv1alpha1.ServiceExport{}
		getErr := rootClient.Get(context.TODO(), client.ObjectKey{Namespace: export.Namespace, Name: export.Name}, newExport)
		if getErr == nil {
			//Make a copy, so we don't mutate the shared cache
			export = newExport.DeepCopy()
		} else {
			if apierrors.IsNotFound(getErr) {
				return nil
			} else {
				klog.Errorf("Failed to get serviceExport %s/%s: %v", export.Namespace, export.Name, getErr)
			}
		}

		return updateErr
	})
}

func (c *ServiceExportController) syncServiceExport(export *mcsv1alpha1.ServiceExport) error {
	var err error
	selector := labels.SelectorFromSet(
		map[string]string{
			utils.ServiceKey: export.Name,
		},
	)
	epsList := &discoveryv1.EndpointSliceList{}
	err = c.RootClient.List(context.TODO(), epsList, &client.ListOptions{
		Namespace:     export.Namespace,
		LabelSelector: selector,
	})
	if err != nil {
		klog.Errorf("List endpointSlice in %s failed, Error: %v", export.Namespace, err)
		return err
	}

	endpointSlices := epsList.Items
	for i := range endpointSlices {
		newEps := &endpointSlices[i]
		if newEps.DeletionTimestamp != nil {
			klog.V(4).Infof("EndpointSlice %s/%s is deleting and does not need to remove serviceExport annotation", export.Namespace, newEps.Name)
			continue
		}

		err = c.updateEndpointSlice(newEps, c.RootClient, func(eps *discoveryv1.EndpointSlice) {
			helper.AddEndpointSliceAnnotation(eps, utils.ServiceExportLabelKey, utils.MCSLabelValue)
		})
		if err != nil {
			klog.Errorf("Update endpointSlice (%s/%s) failed, Error: %v", export.Namespace, newEps.Name, err)
			return err
		}
	}

	c.EventRecorder.Event(export, corev1.EventTypeNormal, "Synced", "serviceExport has been synced to endpointSlice's annotation successfully")
	return nil
}

func (c *ServiceExportController) ensureFinalizer(export *mcsv1alpha1.ServiceExport) (reconcile.Result, error) {
	if controllerutil.ContainsFinalizer(export, utils.MCSFinalizer) {
		return controllerruntime.Result{}, nil
	}

	err := c.updateServiceExport(export, c.RootClient, func(export *mcsv1alpha1.ServiceExport) {
		controllerutil.AddFinalizer(export, utils.MCSFinalizer)
	})
	if err != nil {
		klog.Errorf("Update serviceExport (%s/%s) failed, Error: %v", export.Namespace, export.Name, err)
		return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *ServiceExportController) removeFinalizer(export *mcsv1alpha1.ServiceExport) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(export, utils.MCSFinalizer) {
		return controllerruntime.Result{}, nil
	}

	err := c.updateServiceExport(export, c.RootClient, func(export *mcsv1alpha1.ServiceExport) {
		controllerutil.RemoveFinalizer(export, utils.MCSFinalizer)
	})
	if err != nil {
		klog.Errorf("Update serviceExport %s/%s's finalizer failed, Error: %v", export.Namespace, export.Name, err)
		return controllerruntime.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	return controllerruntime.Result{}, nil
}
