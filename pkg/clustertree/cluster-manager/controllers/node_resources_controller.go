package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "node-resources-controller"
	RequeueTime    = 10 * time.Second
)

type NodeResourcesController struct {
	Client        client.Client
	Master        client.Client
	EventRecorder record.EventRecorder
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
		return false
	},
}

func (c *NodeResourcesController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Node{}, builder.WithPredicates(predicatesFunc)).
		Complete(c)
}

func (c *NodeResourcesController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("============ %s starts to reconcile %s ============", ControllerName, request.Name)
	defer func() {
		klog.Infof("============ %s has been reconciled =============", request.Name)
	}()

	return reconcile.Result{}, nil
}
