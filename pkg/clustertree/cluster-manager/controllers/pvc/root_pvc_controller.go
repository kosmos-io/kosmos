package pvc

import (
	"context"
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mergetypes "k8s.io/apimachinery/pkg/types"
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

	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	RootPVCControllerName = "root-pvc-controller"
	RootPVCRequeueTime    = 10 * time.Second
)

type RootPVCController struct {
	LeafClient    client.Client
	RootClient    client.Client
	LeafClientSet kubernetes.Interface
}

func (r *RootPVCController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pvc := &v1.PersistentVolumeClaim{}
	var deletePVCInClient bool
	err := r.RootClient.Get(ctx, request.NamespacedName, pvc)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
		}
		err = r.LeafClient.Get(ctx, request.NamespacedName, pvc)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("get pvc from leaf cluster failed, error: %v", err)
				return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
			}
			klog.V(4).Infof("leaf pvc namespace: %q, name: %q not exist", request.NamespacedName.Namespace,
				request.NamespacedName.Name)
			return reconcile.Result{}, nil
		}
		deletePVCInClient = true
	}

	if deletePVCInClient || pvc.DeletionTimestamp != nil {
		if err = r.LeafClient.Delete(ctx, pvc); err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("delete pvc from leaf cluster failed, error: %v", err)
				return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
			}
		}
		klog.V(4).Infof("leaf pvc namespace: %q, name: %q has deleted", request.NamespacedName.Namespace,
			request.NamespacedName.Name)
		return reconcile.Result{}, nil
	}

	pvcOld := &v1.PersistentVolumeClaim{}
	err = r.LeafClient.Get(ctx, request.NamespacedName, pvcOld)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Warningf("get pvc from leaf cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
		}
		return reconcile.Result{}, nil
	}

	if !utils.IsObjectGlobal(&pvcOld.ObjectMeta) {
		return reconcile.Result{}, nil
	}

	if reflect.DeepEqual(pvcOld.Spec, pvc.Spec) {
		return reconcile.Result{}, nil
	}

	patch, err := utils.CreateMergePatch(pvcOld, pvc)
	if err != nil {
		klog.Errorf("patch pvc error: %v", err)
		return reconcile.Result{}, err
	}
	_, err = r.LeafClientSet.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(ctx,
		pvc.Name, mergetypes.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("patch pvc namespace: %q, name: %q from root cluster failed, error: %v",
			request.NamespacedName.Namespace, request.NamespacedName.Name, err)
		return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}

func (r *RootPVCController) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(RootPVCControllerName).
		WithOptions(controller.Options{}).
		For(&v1.PersistentVolumeClaim{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return false
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
		})).
		Complete(r)
}
