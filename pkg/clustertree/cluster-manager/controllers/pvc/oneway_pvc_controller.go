package pvc

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	controllerName = "oneway-pvc-controller"
	requeueTime    = 10 * time.Second
)

type OnewayPVCController struct {
	Root                    client.Client
	RootDynamic             dynamic.Interface
	GlobalLeafManager       leafUtils.LeafResourceManager
	GlobalLeafClientManager leafUtils.LeafClientResourceManager
}

func (c *OnewayPVCController) SetupWithManager(mgr manager.Manager) error {
	predicatesFunc := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			curr := createEvent.Object.(*corev1.PersistentVolumeClaim)
			return podutils.IsOneWayPVC(curr)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			curr := updateEvent.ObjectNew.(*corev1.PersistentVolumeClaim)
			return podutils.IsOneWayPVC(curr)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			curr := deleteEvent.Object.(*corev1.PersistentVolumeClaim)
			return podutils.IsOneWayPVC(curr)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{}).
		For(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(predicatesFunc)).
		Complete(c)
}

func (c *OnewayPVCController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", controllerName, request.Name)
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled =============", request.Name)
	}()

	rootPVC := &corev1.PersistentVolumeClaim{}
	pvcErr := c.Root.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: request.Name}, rootPVC)
	if pvcErr != nil && !errors.IsNotFound(pvcErr) {
		klog.Errorf("get pvc %s error: %v", request.Name, pvcErr)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}

	// volumePath has the same name with pvc
	vp, err := c.RootDynamic.Resource(podutils.VolumePathGVR).Get(ctx, request.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("vp %s not found", request.Name)
			return reconcile.Result{RequeueAfter: requeueTime}, nil
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

	lcr, err := c.leafClientResource(leaf)
	if err != nil {
		klog.Errorf("Failed to get leaf client resource %v", leaf.Cluster.Name)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if pvcErr != nil && errors.IsNotFound(pvcErr) ||
		!rootPVC.DeletionTimestamp.IsZero() {
		return c.clearLeafPVC(ctx, leaf, lcr, rootPVC)
	}

	return c.ensureLeafPVC(ctx, leaf, lcr, rootPVC)
}

func (c *OnewayPVCController) clearLeafPVC(ctx context.Context, leaf *leafUtils.LeafResource, leafClient *leafUtils.LeafClientResource, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (c *OnewayPVCController) ensureLeafPVC(ctx context.Context, leaf *leafUtils.LeafResource, leafClient *leafUtils.LeafClientResource, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	clusterName := leaf.Cluster.Name
	newPVC := pvc.DeepCopy()

	anno := newPVC.GetAnnotations()
	anno = utils.AddResourceClusters(anno, leaf.Cluster.Name)
	anno[utils.KosmosGlobalLabel] = "true"
	newPVC.SetAnnotations(anno)

	oldPVC := &corev1.PersistentVolumeClaim{}
	err := leafClient.Client.Get(ctx, types.NamespacedName{
		Name:      newPVC.Name,
		Namespace: newPVC.Namespace,
	}, oldPVC)
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("get pvc from cluster %s error: %v, will requeue", leaf.Cluster.Name, err)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}

	// create
	if err != nil && errors.IsNotFound(err) {
		newPVC.UID = ""
		newPVC.ResourceVersion = ""
		if err = leafClient.Client.Create(ctx, newPVC); err != nil && !errors.IsAlreadyExists(err) {
			klog.Errorf("create pv to cluster %s error: %v, will requeue", clusterName, err)
			return reconcile.Result{RequeueAfter: requeueTime}, nil
		}
		return reconcile.Result{}, nil
	}

	// update
	newPVC.ResourceVersion = oldPVC.ResourceVersion
	newPVC.UID = oldPVC.UID
	if utils.IsPVCEqual(oldPVC, newPVC) {
		return reconcile.Result{}, nil
	}
	patch, err := utils.CreateMergePatch(oldPVC, newPVC)
	if err != nil {
		klog.Errorf("patch pv error: %v", err)
		return reconcile.Result{}, err
	}
	_, err = leafClient.Clientset.CoreV1().PersistentVolumeClaims(newPVC.Namespace).Patch(ctx, newPVC.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("patch pvc %s to %s cluster failed, error: %v", newPVC.Name, leaf.Cluster.Name, err)
		return reconcile.Result{RequeueAfter: requeueTime}, nil
	}
	return reconcile.Result{}, nil
}

func (c *OnewayPVCController) leafClientResource(lr *leafUtils.LeafResource) (*leafUtils.LeafClientResource, error) {
	actualClusterName := leafUtils.GetActualClusterName(lr.Cluster)
	lcr, err := c.GlobalLeafClientManager.GetLeafResource(actualClusterName)
	if err != nil {
		return nil, fmt.Errorf("get leaf client resource err: %v", err)
	}
	return lcr, nil
}
