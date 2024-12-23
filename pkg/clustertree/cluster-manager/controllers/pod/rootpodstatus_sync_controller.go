package pod

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
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
	PodSyncControllerName = "pod-sync-controller"
)

// feature:sync rootpod status from leafpod status when rootpod status modified by others(not from leafpod)
type RootPodSyncReconciler struct {
	RootClient              client.Client
	GlobalLeafManager       leafUtils.LeafResourceManager
	GlobalLeafClientManager leafUtils.LeafClientResourceManager
}

func (c *RootPodSyncReconciler) SetupWithManager(mgr manager.Manager) error {
	skipFunc := func(obj client.Object) bool {
		p := obj.(*corev1.Pod)
		return podutils.IsKosmosPod(p)
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(PodSyncControllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Pod{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				oldPod := updateEvent.ObjectOld.(*corev1.Pod)
				newPod := updateEvent.ObjectNew.(*corev1.Pod)
				if !skipFunc(updateEvent.ObjectNew) {
					return false
				}
				return !reflect.DeepEqual(oldPod.Status, newPod.Status)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(c)
}

func (c *RootPodSyncReconciler) leafClientResource(lr *leafUtils.LeafResource) (*leafUtils.LeafClientResource, error) {
	actualClusterName := leafUtils.GetActualClusterName(lr.Cluster)
	lcr, err := c.GlobalLeafClientManager.GetLeafResource(actualClusterName)
	if err != nil {
		return nil, fmt.Errorf("get leaf client resource err: %v", err)
	}
	return lcr, nil
}

func (c *RootPodSyncReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", PodSyncControllerName, request.Name)

	var rootpod corev1.Pod
	if err := c.RootClient.Get(ctx, request.NamespacedName, &rootpod); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("pods not found, %s", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	//get LeafResource
	lr, err := c.GlobalLeafManager.GetLeafResourceByNodeName(rootpod.Spec.NodeName)
	if err != nil {
		klog.Errorf("Failed to get leaf client for  %s", rootpod.Spec.NodeName)
		return reconcile.Result{}, nil
	}

	lcr, err := c.leafClientResource(lr)
	if err != nil {
		klog.Errorf("Failed to get leaf client resource %v", lr.Cluster.Name)
		return reconcile.Result{}, nil
	}

	leafPod := &corev1.Pod{}
	err = lcr.Client.Get(ctx, request.NamespacedName, leafPod)
	if err != nil {
		klog.Errorf("Failed to get leaf pod  %v", leafPod.Name)
		return reconcile.Result{}, nil
	}

	if podutils.IsKosmosPod(leafPod) && !reflect.DeepEqual(rootpod.Status, leafPod.Status) {
		rPodCopy := rootpod.DeepCopy()
		rPodCopy.Status = leafPod.Status
		podutils.FitObjectMeta(&rPodCopy.ObjectMeta)
		if err := c.RootClient.Status().Update(ctx, rPodCopy); err != nil {
			klog.Errorf("error while updating rootpod status in kubernetes, %s", err)
			return reconcile.Result{}, nil
		}
		klog.Infof("update rootpod %s status success", leafPod.Name)
	}
	klog.V(4).Infof("============ %s has been reconciled %s ============", PodSyncControllerName, request.Name)
	return reconcile.Result{}, nil
}
