package mcs

import (
	"context"

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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/kosmos.io/kosmos/pkg/utils"
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
}

func (c *ServiceExportController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", ServiceExportControllerName, request.NamespacedName.String())
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled =============", request.NamespacedName.String())
	}()

	var shouldDelete bool
	serviceExport := &mcsv1alpha1.ServiceExport{}
	if err := c.RootClient.Get(ctx, request.NamespacedName, serviceExport); err != nil {
		if !apierrors.IsNotFound(err) {
			return controllerruntime.Result{Requeue: true}, err
		}
		shouldDelete = true
	}

	// The serviceExport is being deleted, in which case we should clear endpointSlice.
	if shouldDelete || !serviceExport.DeletionTimestamp.IsZero() {
		if err := c.removeAnnotation(ctx, request.Namespace, request.Name); err != nil {
			return controllerruntime.Result{Requeue: true}, err
		}
		return controllerruntime.Result{}, nil
	}

	err := c.syncServiceExport(ctx, serviceExport)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
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

func (c *ServiceExportController) removeAnnotation(ctx context.Context, namespace, name string) error {
	var err error
	selector := labels.SelectorFromSet(
		map[string]string{
			utils.ServiceKey: name,
		},
	)
	epsList := &discoveryv1.EndpointSliceList{}
	err = c.RootClient.List(ctx, epsList, &client.ListOptions{
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
		helper.RemoveAnnotation(newEps, utils.ServiceExportLabelKey)
		err = c.updateEndpointSlice(ctx, newEps, c.RootClient)
		if err != nil {
			klog.Errorf("Update endpointSlice (%s/%s) failed, Error: %v", namespace, newEps.Name, err)
			return err
		}
	}
	return nil
}

func (c *ServiceExportController) updateEndpointSlice(ctx context.Context, eps *discoveryv1.EndpointSlice, rootClient client.Client) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updateErr := rootClient.Update(ctx, eps)
		if updateErr == nil {
			return nil
		}

		newEps := &discoveryv1.EndpointSlice{}
		key := types.NamespacedName{
			Namespace: eps.Namespace,
			Name:      eps.Name,
		}
		getErr := rootClient.Get(ctx, key, newEps)
		if getErr == nil {
			//Make a copy, so we don't mutate the shared cache
			eps = newEps.DeepCopy()
		} else {
			klog.Errorf("Failed to get updated endpointSlice %s/%s: %v", eps.Namespace, eps.Name, getErr)
		}

		return updateErr
	})
}

func (c *ServiceExportController) syncServiceExport(ctx context.Context, export *mcsv1alpha1.ServiceExport) error {
	var err error
	selector := labels.SelectorFromSet(
		map[string]string{
			utils.ServiceKey: export.Name,
		},
	)
	epsList := &discoveryv1.EndpointSliceList{}
	err = c.RootClient.List(ctx, epsList, &client.ListOptions{
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
		helper.AddEndpointSliceAnnotation(newEps, utils.ServiceExportLabelKey, utils.MCSLabelValue)
		err = c.updateEndpointSlice(ctx, newEps, c.RootClient)
		if err != nil {
			klog.Errorf("Update endpointSlice (%s/%s) failed, Error: %v", export.Namespace, newEps.Name, err)
			return err
		}
	}

	c.EventRecorder.Event(export, corev1.EventTypeNormal, "Synced", "serviceExport has been synced to endpointSlice's annotation successfully")
	return nil
}
