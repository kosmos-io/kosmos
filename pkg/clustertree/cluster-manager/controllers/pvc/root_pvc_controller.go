package pvc

import (
	"context"
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mergetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	RootPVCControllerName = "root-pvc-controller"
	RootPVCRequeueTime    = 10 * time.Second
)

type RootPVCController struct {
	RootClient        client.Client
	GlobalLeafManager leafUtils.LeafResourceManager
}

func (r *RootPVCController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pvc := &v1.PersistentVolumeClaim{}
	err := r.RootClient.Get(ctx, request.NamespacedName, pvc)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Warningf("get pvc from root cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
		}
		return reconcile.Result{}, nil
	}

	clusters := utils.ListResourceOwnersAnnotations(pvc.Annotations)
	if len(clusters) == 0 {
		klog.Warningf("pvc leaf %q: %q doesn't existed", request.NamespacedName.Namespace, request.NamespacedName.Name)
		return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
	}

	lr, err := r.GlobalLeafManager.GetLeafResource(clusters[0])
	if err != nil {
		klog.Warningf("pvc leaf %q: %q doesn't existed in LeafResources", request.NamespacedName.Namespace, request.NamespacedName.Name)
		return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
	}

	pvcOld := &v1.PersistentVolumeClaim{}
	err = lr.Client.Get(ctx, request.NamespacedName, pvcOld)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Warningf("get pvc from leaf cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
		}
		// TODO Create?
		return reconcile.Result{}, nil
	}

	/*	if !utils.IsObjectGlobal(&pvcOld.ObjectMeta) {
		return reconcile.Result{}, nil
	}*/

	if reflect.DeepEqual(pvcOld.Spec, pvc.Spec) {
		return reconcile.Result{}, nil
	}

	patch, err := utils.CreateMergePatch(pvcOld, pvc)
	if err != nil {
		klog.Errorf("patch pvc error: %v", err)
		return reconcile.Result{}, err
	}
	_, err = lr.Clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(ctx,
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
				if deleteEvent.DeleteStateUnknown {
					//TODO ListAndDelete
					klog.Warningf("missing delete pvc root event %q: %q", deleteEvent.Object.GetNamespace(), deleteEvent.Object.GetName())
					return false
				}

				pvc := deleteEvent.Object.(*v1.PersistentVolumeClaim)
				clusters := utils.ListResourceOwnersAnnotations(pvc.Annotations)
				if len(clusters) == 0 {
					klog.Warningf("pvc leaf %q: %q doesn't existed", deleteEvent.Object.GetNamespace(), deleteEvent.Object.GetName())
					return false
				}

				lr, err := r.GlobalLeafManager.GetLeafResource(clusters[0])
				if err != nil {
					klog.Warningf("pvc leaf %q: %q doesn't existed in LeafResources", deleteEvent.Object.GetNamespace(),
						deleteEvent.Object.GetName())
					return false
				}

				if err = lr.Clientset.CoreV1().PersistentVolumeClaims(deleteEvent.Object.GetNamespace()).Delete(context.TODO(),
					deleteEvent.Object.GetName(), metav1.DeleteOptions{}); err != nil {
					if !errors.IsNotFound(err) {
						klog.Errorf("delete pvc from leaf cluster failed, %q: %q, error: %v", deleteEvent.Object.GetNamespace(),
							deleteEvent.Object.GetName(), err)
					}
				}

				return false
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}
