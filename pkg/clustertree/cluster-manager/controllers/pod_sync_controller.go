package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
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
)

// type PodSyncController struct {
// 	leafClient kubernetes.Interface
// 	rootClient kubernetes.Interface
// 	root       client.Client
// 	nodes      []*corev1.Node
// }

type PodSyncController struct {
	RootClient              client.Client
	LeafModelHandler        leafUtils.LeafModelHandler
	GlobalLeafManager       leafUtils.LeafResourceManager
	GlobalLeafClientManager leafUtils.LeafClientResourceManager
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
				currReady := util.IsNodeReady(curr.Status.Conditions)
				oldReady := util.IsNodeReady(old.Status.Conditions)
				return currReady == oldReady
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

// func (c *PodSyncController) syncPodStatus(ctx context.Context) error {
// 	err := c.updatePodStatus(ctx)
// 	if err != nil {
// 		klog.Errorf(err.Error())
// 		return err
// 	}
// 	return nil
// }

func (c *PodSyncController) updatePodStatus(ctx context.Context) error {
	//find cluster in leaf and use pod to update
	clusters := c.GlobalLeafManager.ListClusters()
	for _, cluster := range clusters {
		if c.GlobalLeafManager.HasCluster(cluster) {
			lr, err := c.GlobalLeafManager.GetLeafResource(cluster)
			if err != nil {
				klog.Errorf("get lr(cluster: %s) err: %v", cluster, err)
				return err
			}
			lcr, err := c.leafClientResource(lr)
			if err != nil {
				klog.Errorf("Failed to get leaf client resource %v", lr.Cluster.Name)
				return err
			}

			Selector := labels.SelectorFromSet(
				map[string]string{
					utils.KosmosPodLabel: "true",
				})
			pods, err := lcr.Clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
				LabelSelector: Selector.String(),
			})
			if err != nil {
				klog.Errorf("Could not list kosmos pods in leaf cluster,Error: %v", err)
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
						if err := c.RootClient.Get(ctx, types.NamespacedName{Name: leafpod.Name, Namespace: leafpod.Namespace}, rootpod); err != nil {
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
							if err := c.RootClient.Status().Update(ctx, rPodCopy); err != nil && !apierrors.IsNotFound(err) {
								klog.Errorf("error while updating pod status in kubernetes, %s", err)
								return err
							}
						}
						return nil
					})
					if err != nil {
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
					taskErr = fmt.Errorf("%v; %v", taskErr, err)
				}
			}
			if taskErr != nil {
				return taskErr
			}
		}
	}
	return nil
}

func (r *PodSyncController) leafClientResource(lr *leafUtils.LeafResource) (*leafUtils.LeafClientResource, error) {
	actualClusterName := leafUtils.GetActualClusterName(lr.Cluster)
	lcr, err := r.GlobalLeafClientManager.GetLeafResource(actualClusterName)
	if err != nil {
		return nil, fmt.Errorf("get leaf client resource err: %v", err)
	}
	return lcr, nil
}

func (c *PodSyncController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", PodSyncControllerName, request.Name)

	//find node in root
	nodeInRoot := &corev1.Node{}
	if err := c.RootClient.Get(ctx, request.NamespacedName, nodeInRoot); err != nil {
		if !errors.IsNotFound(err) {
			klog.Errorf("get root node failed, error: %v", err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
	}

	if !util.IsNodeReady(nodeInRoot.Status.Conditions) {
		klog.Infof("Node %s is not ready; starting syncPodStatus.", nodeInRoot.Name)
		err := c.updatePodStatus(ctx)
		if err != nil {
			klog.Errorf("Failed to sync pod status: %v", err)
			return reconcile.Result{}, err
		}
	}

	// clusterName := nodeInRoot.Annotations[utils.KosmosNodeOwnedByClusterAnnotations]
	// if clusterName == "" {
	// 	klog.Warningf("node %s is kosmos node, but node's %s annotation is empty, will requeue", nodeInRoot.Name, utils.KosmosNodeOwnedByClusterAnnotations)
	// 	return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	// }

	// lr, err := c.GlobalLeafManager.GetLeafResource(clusterName)
	// if err != nil {
	// 	klog.Warningf("get leafManager for cluster %s failed, error: %v, will requeue", clusterName, err)
	// 	return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	// }

	// lcr, err := c.leafClientResource(lr)
	// if err != nil {
	// 	klog.Errorf("Failed to get leaf client resource %v", lr.Cluster.Name)
	// 	return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	// }
	// klog.Infof("LeafResource for cluster %s: %+v", clusterName, lcr)

	// pods, err := lcr.Clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	// if err != nil {
	// 	klog.Errorf("Could not list pods in leaf cluster,Error: %v", err)
	// }

	// for _, pod := range pods.Items {
	// 	klog.Infof("Pod Name: %s, Namespace: %s, Phase: %s",
	// 		pod.Name, pod.Namespace, pod.Status.Phase)
	// }
	// clusterName, exists := nodeInRoot.Annotations["kosmos-io/owned-by-cluster"]
	// if !exists {
	// 	klog.Errorf("annotation 'kosmos-io/owned-by-cluster' not found on node %s", nodeInRoot.Name)
	// 	return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	// }

	klog.V(4).Infof("============ %s has been reconciled =============", request.NamespacedName)
	return reconcile.Result{}, nil
}
