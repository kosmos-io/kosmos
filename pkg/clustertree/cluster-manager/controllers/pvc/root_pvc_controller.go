package pvc

import (
	"context"
	"fmt"
	"reflect"

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
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	RootPVCControllerName = "root-pvc-controller"
)

type RootPVCController struct {
	RootClient              client.Client
	GlobalLeafManager       leafUtils.LeafResourceManager
	GlobalLeafClientManager leafUtils.LeafClientResourceManager
}

func (r *RootPVCController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pvc := &v1.PersistentVolumeClaim{}
	shouldDelete := false
	err := r.RootClient.Get(ctx, request.NamespacedName, pvc)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Warningf("get pvc from root cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
		shouldDelete = true
		pvc.Namespace = request.Namespace
		pvc.Name = request.Name
	}

	if !pvc.DeletionTimestamp.IsZero() || shouldDelete {
		return r.cleanupPvc(pvc)
	}

	clusters := utils.ListResourceClusters(pvc.Annotations)
	if len(clusters) == 0 {
		klog.V(4).Infof("pvc leaf %q: %q doesn't existed", request.NamespacedName.Namespace, request.NamespacedName.Name)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	lr, err := r.GlobalLeafManager.GetLeafResource(clusters[0])
	if err != nil {
		klog.Warningf("pvc leaf %q: %q doesn't existed in LeafResources", request.NamespacedName.Namespace, request.NamespacedName.Name)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	lcr, err := r.leafClientResource(lr)
	if err != nil {
		klog.Errorf("Failed to get leaf client resource %v", lr.Cluster.Name)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	pvcOld := &v1.PersistentVolumeClaim{}
	err = lcr.Client.Get(ctx, request.NamespacedName, pvcOld)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Warningf("get pvc from leaf cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
		// TODO Create?
		return reconcile.Result{}, nil
	}

	/*	if !utils.IsObjectGlobal(&pvcOld.ObjectMeta) {
		return reconcile.Result{}, nil
	}*/

	if reflect.DeepEqual(pvcOld.Spec.Resources.Requests, pvc.Spec.Resources.Requests) {
		return reconcile.Result{}, nil
	}

	pvc.Annotations = pvcOld.Annotations
	pvc.ObjectMeta.UID = pvcOld.ObjectMeta.UID
	pvc.ObjectMeta.ResourceVersion = ""
	pvc.ObjectMeta.OwnerReferences = pvcOld.ObjectMeta.OwnerReferences
	patch, err := utils.CreateMergePatch(pvcOld, pvc)
	if err != nil {
		klog.Errorf("patch pvc error: %v", err)
		return reconcile.Result{}, err
	}

	_, err = lcr.Clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(ctx,
		pvc.Name, mergetypes.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("patch pvc namespace: %q, name: %q from root cluster failed, error: %v",
			request.NamespacedName.Namespace, request.NamespacedName.Name, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
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
				// skip  one way pvc, oneway_pv_controller will handle this PVC
				curr := updateEvent.ObjectNew.(*v1.PersistentVolumeClaim)
				return !podutils.IsOneWayPVC(curr)
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

func (r *RootPVCController) cleanupPvc(pvc *v1.PersistentVolumeClaim) (reconcile.Result, error) {
	clusters := utils.ListResourceClusters(pvc.Annotations)
	if len(clusters) == 0 {
		klog.V(4).Infof("pvc leaf %q: %q doesn't existed", pvc.GetNamespace(), pvc.GetName())
		return reconcile.Result{}, nil
	}

	lr, err := r.GlobalLeafManager.GetLeafResource(clusters[0])
	if err != nil {
		klog.Warningf("pvc leaf %q: %q doesn't existed in LeafResources", pvc.GetNamespace(), pvc.GetName())
		return reconcile.Result{}, nil
	}

	lcr, err := r.leafClientResource(lr)
	if err != nil {
		klog.Errorf("Failed to get leaf client resource %v", lr.Cluster.Name)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if err = lcr.Clientset.CoreV1().PersistentVolumeClaims(pvc.GetNamespace()).Delete(context.TODO(), pvc.GetName(), metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			klog.Errorf("delete pvc from leaf cluster failed, %q: %q, error: %v", pvc.GetNamespace(), pvc.GetName(), err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *RootPVCController) leafClientResource(lr *leafUtils.LeafResource) (*leafUtils.LeafClientResource, error) {
	actualClusterName := leafUtils.GetActualClusterName(lr.Cluster)
	lcr, err := r.GlobalLeafClientManager.GetLeafResource(actualClusterName)
	if err != nil {
		return nil, fmt.Errorf("get leaf client resource err: %v", err)
	}
	return lcr, nil
}
