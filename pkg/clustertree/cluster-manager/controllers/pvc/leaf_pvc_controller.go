package pvc

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mergetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	LeafPVCControllerName = "leaf-pvc-controller"
	LeafPVCRequeueTime    = 10 * time.Second
)

type LeafPVCController struct {
	LeafClient    client.Client
	RootClient    client.Client
	RootClientSet kubernetes.Interface
	ClusterName   string
}

func (l *LeafPVCController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pvc := &v1.PersistentVolumeClaim{}
	err := l.LeafClient.Get(ctx, request.NamespacedName, pvc)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Errorf("get pvc from leaf cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: LeafPVCRequeueTime}, nil
		}
		klog.V(4).Infof("leaf pvc namespace: %q, name: %q deleted", request.NamespacedName.Namespace,
			request.NamespacedName.Name)
		return reconcile.Result{}, nil
	}

	rootPVC := &v1.PersistentVolumeClaim{}
	err = l.RootClient.Get(ctx, request.NamespacedName, rootPVC)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{RequeueAfter: LeafPVCRequeueTime}, nil
		}
		klog.Warningf("pvc namespace: %q, name: %q has been deleted from root cluster", request.NamespacedName.Namespace,
			request.NamespacedName.Name)
		return reconcile.Result{}, nil
	}

	pvcCopy := pvc.DeepCopy()
	if reflect.DeepEqual(rootPVC.Status, pvcCopy.Status) {
		return reconcile.Result{}, nil
	}

	//when root pvc is not bound, it's status can't be changed to bound
	if pvcCopy.Status.Phase == v1.ClaimBound {
		err = wait.PollImmediate(500*time.Millisecond, 1*time.Minute, func() (bool, error) {
			if rootPVC.Spec.VolumeName == "" {
				klog.Warningf("pvc namespace: %q, name: %q is not bounded", request.NamespacedName.Namespace,
					request.NamespacedName.Name)
				err = l.RootClient.Get(ctx, request.NamespacedName, rootPVC)
				if err != nil {
					return false, err
				}
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{RequeueAfter: LeafPVCRequeueTime}, nil
			}
			return reconcile.Result{}, nil
		}
	}

	if err = filterPVC(pvcCopy, l.ClusterName); err != nil {
		return reconcile.Result{}, nil
	}

	delete(pvcCopy.Annotations, utils.PVCSelectedNodeKey)
	pvcCopy.ResourceVersion = rootPVC.ResourceVersion
	pvcCopy.OwnerReferences = rootPVC.OwnerReferences
	utils.AddResourceClusters(pvcCopy.Annotations, l.ClusterName)
	pvcCopy.Spec = rootPVC.Spec
	klog.V(4).Infof("rootPVC %+v\n, pvc %+v", rootPVC, pvcCopy)

	patch, err := utils.CreateMergePatch(rootPVC, pvcCopy)
	if err != nil {
		klog.Errorf("patch pvc error: %v", err)
		return reconcile.Result{}, err
	}
	_, err = l.RootClientSet.CoreV1().PersistentVolumeClaims(rootPVC.Namespace).Patch(ctx,
		rootPVC.Name, mergetypes.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("patch pvc namespace: %q, name: %q to root cluster failed, error: %v",
			request.NamespacedName.Namespace, request.NamespacedName.Name, err)
		return reconcile.Result{RequeueAfter: RootPVCRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}

func (l *LeafPVCController) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(LeafPVCControllerName).
		WithOptions(controller.Options{}).
		For(&v1.PersistentVolumeClaim{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				pvc := updateEvent.ObjectOld.(*v1.PersistentVolumeClaim)
				return utils.IsObjectGlobal(&pvc.ObjectMeta)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(l)
}

func filterPVC(leafPVC *v1.PersistentVolumeClaim, nodeName string) error {
	labelSelector := leafPVC.Spec.Selector.DeepCopy()
	leafPVC.Spec.Selector = nil
	leafPVC.ObjectMeta.UID = ""
	leafPVC.ObjectMeta.ResourceVersion = ""
	leafPVC.ObjectMeta.OwnerReferences = nil

	podutils.SetObjectGlobal(&leafPVC.ObjectMeta)
	if labelSelector != nil {
		labelStr, err := json.Marshal(labelSelector)
		if err != nil {
			klog.Errorf("pvc namespace: %q, name: %q marshal label failed", leafPVC.Namespace, leafPVC.Name)
			return err
		}
		leafPVC.Annotations[utils.KosmosPvcLabelSelector] = string(labelStr)
	}
	return nil
}
