package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	PodSyncControllerName = "pod-sync-controller"

	//DefaultpodStatusUpdateInterval = 10 * time.Second
)

type PodSyncController struct {
	leafClient kubernetes.Interface
	rootClient kubernetes.Interface
	root       client.Client
	nodes      []*corev1.Node
	//podstatusInterval time.Duration
}

func NewPodSyncController(leafClient kubernetes.Interface, rootClient kubernetes.Interface, root client.Client, nodes []*corev1.Node) *PodSyncController {
	c := &PodSyncController{
		leafClient: leafClient,
		rootClient: rootClient,
		root:       root,
		nodes:      nodes,
		//podstatusInterval: DefaultpodStatusUpdateInterval,
	}
	return c
}

func IsNodeReady(node *corev1.Node) bool {
	for _, conditon := range node.Status.Conditions {
		if conditon.Type == corev1.NodeReady && conditon.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (c *PodSyncController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(PodSyncControllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Node{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				curr := updateEvent.ObjectNew.(*corev1.Node)
				old := updateEvent.ObjectOld.(*corev1.Node)
				return IsNodeReady(curr) != IsNodeReady(old)
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

// func (c *PodSyncController) Start(ctx context.Context) error {
// 	go wait.UntilWithContext(ctx, c.syncPodStatus, c.podstatusInterval)
// 	<-ctx.Done()
// 	return nil
// }

func (c *PodSyncController) syncPodStatus(ctx context.Context) error {

	err := c.updatePodStatus(ctx)
	if err != nil {
		klog.Errorf(err.Error())
		return err
	}
	return nil
}

func (c *PodSyncController) updatePodStatus(ctx context.Context) error {
	Selector := labels.SelectorFromSet(
		map[string]string{
			utils.KosmosPodLabel: "true",
		})
	pods, err := c.leafClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: Selector.String(),
	})
	if err != nil {
		klog.Errorf("Could not list pods in leaf cluster,Error: %v", err)
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(pods.Items))

	for _, leafpod := range pods.Items {
		wg.Add(1)
		go func(leafpod corev1.Pod) {
			defer wg.Done()
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				rootpod := &corev1.Pod{}
				if err := c.root.Get(ctx, types.NamespacedName{Name: leafpod.Name, Namespace: leafpod.Namespace}, rootpod); err != nil {
					if apierrors.IsNotFound(err) {
						klog.Warningf("Pod %s in namespace %s not found in root cluster, skipping...", leafpod.Name, leafpod.Namespace)
						return nil
					}
					return err
				}
				if podutils.IsKosmosPod(rootpod) && !reflect.DeepEqual(rootpod.Status, leafpod.Status) {
					rPodCopy := rootpod.DeepCopy()
					rPodCopy.Status = leafpod.Status
					podutils.FitObjectMeta(&rPodCopy.ObjectMeta)
					if err := c.root.Status().Update(ctx, rPodCopy); err != nil && !apierrors.IsNotFound(err) {
						klog.V(4).Info(errors.Wrap(err, "error while updating pod status in kubernetes"))
						return err
					}
				}
				return nil
			})
			if err != nil {
				//klog.Errorf("failed to update pod %s/%s, error: %v", leafpod.Namespace, leafpod.Name, err)
				errChan <- fmt.Errorf("failed to update pod %s/%s, error: %v", leafpod.Namespace, leafpod.Name, err)
			}
		}(leafpod)
	}

	wg.Wait()
	close(errChan)

	var taskErr error
	for err := range errChan {

		if taskErr == nil {
			taskErr = err
		} else {
			taskErr = errors.Wrap(err, taskErr.Error())
		}
	}

	if taskErr != nil {
		return taskErr
	}

	return nil
}

func (c *PodSyncController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	klog.V(4).Infof("============ %s starts to reconcile %s ============", PodSyncControllerName, req.Name)

	nodes, err := c.rootClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not list pods in root,Error: %v", err)
	}

	for _, node := range nodes.Items {
		if !IsNodeReady(&node) {
			klog.Infof("Node %s is not ready; starting syncPodStatus.", node.Name)
			err := c.syncPodStatus(ctx)
			if err != nil {
				klog.Errorf("Failed to sync pod status: %v", err)
				return reconcile.Result{}, err
			}
			break
		}
	}

	klog.V(4).Infof("============ %s has been reconciled =============", req.Name)
	return reconcile.Result{}, nil
}
