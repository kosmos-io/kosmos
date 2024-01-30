package controllers

import (
	"context"
	"time"

	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

const (
	PromotePolicyControllerName = "sync-leaf-controller"
	PromotePolicyRequeueTime    = 10 * time.Second
)

// PromotePolicyReconciler reconciles a promotePolicy object
type PromotePolicyController struct {
	client.Client
}

func (r *PromotePolicyController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// todo
	klog.Info("Reconcile")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PromotePolicyController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kosmosv1alpha1.PromotePolicy{}).
		Complete(r)
}
