package pv

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	controllerruntime "sigs.k8s.io/controller-runtime"
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
	controllerName   = "oneway-pv-controller"
	requeueTime      = 10 * time.Second
	quickRequeueTime = 3 * time.Second
	csiDriverName    = "infini.volumepath.csi"
)

var VolumePathGVR = schema.GroupVersionResource{
	Version:  "v1alpha1",
	Group:    "lvm.infinilabs.com",
	Resource: "volumepaths",
}

type OnewayPVController struct {
	Root              client.Client
	RootDynamic       dynamic.Interface
	GlobalLeafManager leafUtils.LeafResourceManager
}

func (c *OnewayPVController) SetupWithManager(mgr manager.Manager) error {
	predicatesFunc := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			curr := createEvent.Object.(*corev1.PersistentVolume)
			return curr.Spec.CSI != nil && curr.Spec.CSI.Driver == csiDriverName
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			curr := updateEvent.ObjectNew.(*corev1.PersistentVolume)
			return curr.Spec.CSI != nil && curr.Spec.CSI.Driver == csiDriverName
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			curr := deleteEvent.Object.(*corev1.PersistentVolume)
			return curr.Spec.CSI != nil && curr.Spec.CSI.Driver == csiDriverName
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{}).
		For(&corev1.PersistentVolume{}, builder.WithPredicates(predicatesFunc)).
		Complete(c)
}

func (c *OnewayPVController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", controllerName, request.Name)
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled =============", request.Name)
	}()

	pv := &corev1.PersistentVolume{}
	pvErr := c.Root.Get(ctx, types.NamespacedName{Name: request.Name}, pv)
	if pvErr != nil && !errors.IsNotFound(pvErr) {
		klog.Errorf("get pv %s error: %v", request.Name, pvErr)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}

	// volumePath has the same name with pv
	vp, err := c.RootDynamic.Resource(VolumePathGVR).Get(ctx, request.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Warningf("vp %s not found", request.Name)
			return reconcile.Result{}, nil
		}
		klog.Errorf("get volumePath %s error: %v", request.Name, err)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}

	nodeName, _, _ := unstructured.NestedString(vp.Object, "spec", "node")
	if nodeName == "" {
		klog.Warningf("vp %s's nodeName is empty, skip", request.Name)
		return reconcile.Result{}, nil
	}

	node := &corev1.Node{}
	err = c.Root.Get(ctx, types.NamespacedName{Name: nodeName}, node)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Warningf("cannot find node %s, error: %v", nodeName, err)
			return reconcile.Result{}, nil
		}
		klog.Warningf("get node %s error: %v, will requeue", nodeName, err)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}

	if !utils.IsKosmosNode(node) {
		return reconcile.Result{}, nil
	}

	clusterName := node.Annotations[utils.KosmosNodeOwnedByClusterAnnotations]
	if clusterName == "" {
		klog.Warningf("node %s is kosmos node, but node's %s annotation is empty, will requeue", node.Name, utils.KosmosNodeOwnedByClusterAnnotations)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}

	leaf, err := c.GlobalLeafManager.GetLeafResource(clusterName)
	if err != nil {
		klog.Warningf("get leafManager for cluster %s failed, error: %v, will requeue", clusterName, err)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}

	if pvErr != nil && errors.IsNotFound(pvErr) ||
		!pv.DeletionTimestamp.IsZero() {
		return c.clearLeafPV(ctx, leaf, pv)
	}

	return c.ensureLeafPV(ctx, leaf, pv)
}

func (c *OnewayPVController) clearLeafPV(ctx context.Context, leaf *leafUtils.LeafResource, pv *corev1.PersistentVolume) (reconcile.Result, error) {
	err := leaf.Clientset.CoreV1().PersistentVolumes().Delete(ctx, pv.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("delete pv %s in %s cluster failed, error: %v", pv.Name, leaf.ClusterName, err)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}
	return reconcile.Result{}, nil
}

func (c *OnewayPVController) ensureLeafPV(ctx context.Context, leaf *leafUtils.LeafResource, pv *corev1.PersistentVolume) (reconcile.Result, error) {
	clusterName := leaf.ClusterName
	newPV := pv.DeepCopy()

	pvc := &corev1.PersistentVolumeClaim{}
	err := leaf.Client.Get(ctx, types.NamespacedName{
		Namespace: newPV.Spec.ClaimRef.Namespace,
		Name:      newPV.Spec.ClaimRef.Name,
	}, pvc)
	if err != nil {
		klog.Errorf("get pvc from cluster %s error: %v, will requeue", leaf.ClusterName, err)
		return reconcile.Result{RequeueAfter: quickRequeueTime}, nil
	}

	newPV.Spec.ClaimRef.ResourceVersion = pvc.ResourceVersion
	newPV.Spec.ClaimRef.UID = pvc.UID

	anno := newPV.GetAnnotations()
	anno = utils.AddResourceClusters(anno, leaf.ClusterName)
	anno[utils.KosmosGlobalLabel] = "true"
	newPV.SetAnnotations(anno)

	oldPV := &corev1.PersistentVolume{}
	err = leaf.Client.Get(ctx, types.NamespacedName{
		Name: newPV.Name,
	}, oldPV)
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("get pv from cluster %s error: %v, will requeue", leaf.ClusterName, err)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}

	// create
	if err != nil && errors.IsNotFound(err) {
		newPV.UID = ""
		newPV.ResourceVersion = ""
		if err = leaf.Client.Create(ctx, newPV); err != nil && !errors.IsAlreadyExists(err) {
			klog.Errorf("create pv to cluster %s error: %v, will requeue", clusterName, err)
			return reconcile.Result{RequeueAfter: requeueTime}, nil
		}
		return reconcile.Result{}, nil
	}

	// update
	newPV.ResourceVersion = oldPV.ResourceVersion
	newPV.UID = oldPV.UID
	if utils.IsPVEqual(oldPV, newPV) {
		return reconcile.Result{}, nil
	}
	patch, err := utils.CreateMergePatch(oldPV, newPV)
	if err != nil {
		klog.Errorf("patch pv error: %v", err)
		return reconcile.Result{}, err
	}
	_, err = leaf.Clientset.CoreV1().PersistentVolumes().Patch(ctx, newPV.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("patch pv %s to %s cluster failed, error: %v", newPV.Name, leaf.ClusterName, err)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}
	return reconcile.Result{}, nil
}
