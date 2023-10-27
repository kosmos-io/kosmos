package pv

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	RootPVControllerName = "root-pv-controller"
	RootPVRequeueTime    = 10 * time.Second
)

type RootPVController struct {
	LeafClient    client.Client
	RootClient    client.Client
	LeafClientSet kubernetes.Interface
}

func (r *RootPVController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pv := &v1.PersistentVolume{}
	err := r.RootClient.Get(ctx, request.NamespacedName, pv)
	pvNeedDelete := false
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Errorf("get root failed, pv %q: %v", pv.Name, err)
			return reconcile.Result{RequeueAfter: RootPVRequeueTime}, nil
		}
		err = r.RootClient.Get(ctx, request.NamespacedName, pv)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("get root failed again, pv %q: %v", pv.Name, err)
				return reconcile.Result{RequeueAfter: RootPVRequeueTime}, nil
			}
			pvNeedDelete = true
		}
	}

	if pvNeedDelete || pv.DeletionTimestamp != nil {
		if err = r.LeafClientSet.CoreV1().PersistentVolumes().Delete(ctx, request.NamespacedName.Name, metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("delete root pv , pv %q: %v", pv.Name, err)
				return reconcile.Result{RequeueAfter: RootPVRequeueTime}, nil
			}
		}
		klog.V(4).Infof("leaf pv %q deleted", pv.Name)
	}

	return reconcile.Result{}, nil
}

func (r *RootPVController) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(RootPVControllerName).
		WithOptions(controller.Options{}).
		For(&v1.PersistentVolumeClaim{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return false
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}
