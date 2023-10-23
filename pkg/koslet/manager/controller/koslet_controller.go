package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const KubeletControllerName = "kubelet-controller"

// KubeletController
type KubeletController struct {
	Client        client.Client
	Master        client.Client
	EventRecorder record.EventRecorder
}

func (kc *KubeletController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(kc)
}

func (kc *KubeletController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("============ %s starts to reconcile %s ============", KubeletControllerName, request.NamespacedName.String())
	defer func() {
		klog.Infof("============ %s has been reconciled =============", request.NamespacedName.String())
	}()
	//TODO
	return controllerruntime.Result{}, nil
}
