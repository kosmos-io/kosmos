package pv

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	LeafPVControllerName = "leaf-pv-controller"
)

type LeafPVController struct {
	LeafClient    client.Client
	RootClient    client.Client
	RootClientSet kubernetes.Interface
	ClusterName   string
	IsOne2OneMode bool
}

func (l *LeafPVController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pv := &v1.PersistentVolume{}
	pvNeedDelete := false
	err := l.LeafClient.Get(ctx, request.NamespacedName, pv)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Errorf("get pv from leaf cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
		pvNeedDelete = true
	}

	pvCopy := pv.DeepCopy()
	rootPV := &v1.PersistentVolume{}
	err = l.RootClient.Get(ctx, request.NamespacedName, rootPV)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Errorf("get root pv failed, error: %v", err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}

		if pvNeedDelete || pv.DeletionTimestamp != nil {
			return reconcile.Result{}, nil
		}

		if pvCopy.Spec.ClaimRef != nil {
			tmpPVC := &v1.PersistentVolumeClaim{}
			nn := types.NamespacedName{
				Name:      pvCopy.Spec.ClaimRef.Name,
				Namespace: pvCopy.Spec.ClaimRef.Namespace,
			}
			err := l.LeafClient.Get(ctx, nn, tmpPVC)
			if err != nil {
				if !errors.IsNotFound(err) {
					klog.Errorf("get tmp pvc failed, error: %v", err)
					return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
				}
				klog.Warningf("tmp pvc not exist, error: %v", err)
				return reconcile.Result{}, nil
			}
			if !utils.IsObjectGlobal(&tmpPVC.ObjectMeta) {
				return reconcile.Result{}, nil
			}
		} else {
			klog.Warningf("Can't find pvc for pv, error: %v", err)
			return reconcile.Result{}, nil
		}

		rootPV = pv.DeepCopy()
		filterPV(rootPV, utils.NodeAffinity4RootPV(pv, l.IsOne2OneMode, l.ClusterName))
		nn := types.NamespacedName{
			Name:      rootPV.Spec.ClaimRef.Name,
			Namespace: rootPV.Spec.ClaimRef.Namespace,
		}

		rootPVC := &v1.PersistentVolumeClaim{}
		err := l.RootClient.Get(ctx, nn, rootPVC)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Can't find root pvc failed, error: %v", err)
			}
			return reconcile.Result{}, nil
		}

		rootPV.Spec.ClaimRef.UID = rootPVC.UID
		rootPV.Spec.ClaimRef.ResourceVersion = rootPVC.ResourceVersion
		utils.AddResourceClusters(rootPV.Annotations, l.ClusterName)

		rootPV, err = l.RootClientSet.CoreV1().PersistentVolumes().Create(ctx, rootPV, metav1.CreateOptions{})
		if err != nil || rootPV == nil {
			klog.Errorf("create pv in root cluster failed, error: %v", err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}

		return reconcile.Result{}, nil
	}

	if !utils.HasResourceClusters(rootPV.Annotations, l.ClusterName) {
		klog.Errorf("meet the same name root pv name: %q !", request.NamespacedName.Name)
		return reconcile.Result{}, nil
	}

	if pvNeedDelete || pv.DeletionTimestamp != nil {
		if err = l.RootClientSet.CoreV1().PersistentVolumes().Delete(ctx, request.NamespacedName.Name, metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("delete root pv failed, error: %v", err)
				return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
			}
		}
		klog.V(4).Infof("root pv name: %q deleted", request.NamespacedName.Name)
		return reconcile.Result{}, nil
	}

	filterPV(rootPV, utils.NodeAffinity4RootPV(pv, l.IsOne2OneMode, l.ClusterName))
	if pvCopy.Spec.ClaimRef != nil || rootPV.Spec.ClaimRef == nil {
		nn := types.NamespacedName{
			Name:      pvCopy.Spec.ClaimRef.Name,
			Namespace: pvCopy.Spec.ClaimRef.Namespace,
		}
		rootPVC := &v1.PersistentVolumeClaim{}
		err := l.RootClient.Get(ctx, nn, rootPVC)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Can't find root pvc failed, error: %v", err)
			}
			return reconcile.Result{}, nil
		}

		pvCopy.Spec.ClaimRef.UID = rootPVC.UID
		pvCopy.Spec.ClaimRef.ResourceVersion = rootPVC.ResourceVersion
	}

	klog.V(4).Infof("root pv %+v\n, leaf pv %+v", rootPV, pvCopy)
	pvCopy.Spec.NodeAffinity = rootPV.Spec.NodeAffinity
	pvCopy.UID = rootPV.UID
	pvCopy.ResourceVersion = rootPV.ResourceVersion
	utils.AddResourceClusters(pvCopy.Annotations, l.ClusterName)

	if utils.IsPVEqual(rootPV, pvCopy) {
		return reconcile.Result{}, nil
	}
	patch, err := utils.CreateMergePatch(rootPV, pvCopy)
	if err != nil {
		klog.Errorf("patch pv error: %v", err)
		return reconcile.Result{}, err
	}
	_, err = l.RootClientSet.CoreV1().PersistentVolumes().Patch(ctx, rootPV.Name, mergetypes.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("patch pv namespace: %q, name: %q to root cluster failed, error: %v",
			request.NamespacedName.Namespace, request.NamespacedName.Name, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	return reconcile.Result{}, nil
}

func (l *LeafPVController) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(LeafPVControllerName).
		WithOptions(controller.Options{}).
		For(&v1.PersistentVolume{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				curr := createEvent.Object.(*v1.PersistentVolume)
				return !podutils.IsOneWayPV(curr)
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				curr := updateEvent.ObjectNew.(*v1.PersistentVolume)
				return !podutils.IsOneWayPV(curr)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				curr := deleteEvent.Object.(*v1.PersistentVolume)
				return !podutils.IsOneWayPV(curr)
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(l)
}

func filterPV(pv *v1.PersistentVolume, nodeName string) {
	pv.ObjectMeta.UID = ""
	pv.ObjectMeta.ResourceVersion = ""
	pv.ObjectMeta.OwnerReferences = nil

	if pv.Annotations == nil {
		pv.Annotations = make(map[string]string)
	}
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return
	}

	selectors := pv.Spec.NodeAffinity.Required.NodeSelectorTerms
	for k, v := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		mfs := v.MatchFields
		mes := v.MatchExpressions
		for k, val := range v.MatchFields {
			if val.Key == utils.NodeHostnameValue || val.Key == utils.NodeHostnameValueBeta || val.Key == utils.OpenebsPVNodeLabel {
				val.Values = []string{nodeName}
			}
			mfs[k] = val
		}
		for k, val := range v.MatchExpressions {
			if val.Key == utils.NodeHostnameValue || val.Key == utils.NodeHostnameValueBeta || val.Key == utils.OpenebsPVNodeLabel {
				val.Values = []string{nodeName}
			}
			mes[k] = val
		}
		selectors[k].MatchFields = mfs
		selectors[k].MatchExpressions = mes
	}
	pv.Spec.NodeAffinity.Required.NodeSelectorTerms = selectors
}
