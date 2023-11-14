package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	NodeResourcesControllerName = "node-resources-controller"
	RequeueTime                 = 10 * time.Second
)

type NodeResourcesController struct {
	Leaf              client.Client
	Root              client.Client
	GlobalLeafManager leafUtils.LeafResourceManager
	RootClientset     kubernetes.Interface

	Nodes            []*corev1.Node
	LeafModelHandler leafUtils.LeafModelHandler
	Cluster          *kosmosv1alpha1.Cluster
	EventRecorder    record.EventRecorder
}

var predicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		return true
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		curr := updateEvent.ObjectNew.(*corev1.Node)
		old := updateEvent.ObjectOld.(*corev1.Node)

		if old.Spec.Unschedulable != curr.Spec.Unschedulable ||
			old.DeletionTimestamp != curr.DeletionTimestamp ||
			utils.NodeReady(old) != utils.NodeReady(curr) ||
			!reflect.DeepEqual(old.Status.Allocatable, curr.Status.Allocatable) {
			return true
		}

		return false
	},
	DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
		return true
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *NodeResourcesController) podMapFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		var requests []reconcile.Request
		pod := a.(*corev1.Pod)

		if len(pod.Spec.NodeName) > 0 {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Name: pod.Spec.NodeName,
			}})
		}
		return requests
	}
}

func (c *NodeResourcesController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(NodeResourcesControllerName).
		WithOptions(controller.Options{}).
		For(&corev1.Node{}, builder.WithPredicates(predicatesFunc)).
		Watches(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(c.podMapFunc())).
		Complete(c)
}

func (c *NodeResourcesController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", NodeResourcesControllerName, request.Name)
	defer func() {
		klog.V(4).Infof("============ %s has been reconciled =============", request.Name)
	}()

	for _, rootNode := range c.Nodes {
		nodeInRoot := &corev1.Node{}
		err := c.Root.Get(ctx, types.NamespacedName{Name: rootNode.Name}, nodeInRoot)
		if err != nil {
			klog.Errorf("Could not get node in root cluster,Error: %v", err)
			return reconcile.Result{
				Requeue:      true,
				RequeueAfter: RequeueTime,
			}, fmt.Errorf("cannot get node while update nodeInRoot resources %s, err: %v", rootNode.Name, err)
		}

		nodesInLeaf, err := c.LeafModelHandler.GetLeafNodes(ctx, rootNode)
		if err != nil {
			klog.Errorf("Could not get node in leaf cluster %s,Error: %v", c.Cluster.Name, err)
			return controllerruntime.Result{
				RequeueAfter: RequeueTime,
			}, err
		}

		pods, err := c.LeafModelHandler.GetLeafPods(ctx, rootNode)
		if err != nil {
			klog.Errorf("Could not list pod in leaf cluster %s,Error: %v", c.Cluster.Name, err)
			return controllerruntime.Result{
				RequeueAfter: RequeueTime,
			}, err
		}

		clusterResources := utils.CalculateClusterResources(nodesInLeaf, pods)
		clone := nodeInRoot.DeepCopy()
		clone.Status.Allocatable = clusterResources
		clone.Status.Capacity = clusterResources
		clone.Status.Conditions = utils.NodeConditions()

		// Node2Node mode should sync leaf node's labels and annotations to root nodeInRoot
		if c.LeafModelHandler.GetLeafModelType() == leafUtils.DispersionModel {
			getNode := func(nodes *corev1.NodeList) *corev1.Node {
				for _, nodeInLeaf := range nodes.Items {
					if nodeInLeaf.Name == rootNode.Name {
						return &nodeInLeaf
					}
				}
				return nil
			}
			node := getNode(nodesInLeaf)
			if node != nil {
				clone.Labels = mergeMap(clone.GetLabels(), node.GetLabels())
				clone.Annotations = mergeMap(clone.GetAnnotations(), node.GetAnnotations())
				clone.Spec = node.Spec
				clone.Spec.Taints = rootNode.Spec.Taints
				clone.Status = node.Status
				clone.Status.Addresses = leafUtils.GetAddress()
			}
		}

		patch, err := utils.CreateMergePatch(nodeInRoot, clone)
		if err != nil {
			klog.Errorf("Could not CreateMergePatch,Error: %v", err)
			return reconcile.Result{}, err
		}

		if _, err = c.RootClientset.CoreV1().Nodes().Patch(ctx, rootNode.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
			return reconcile.Result{
				RequeueAfter: RequeueTime,
			}, fmt.Errorf("failed to patch node resources: %v, will requeue", err)
		}

		if _, err = c.RootClientset.CoreV1().Nodes().PatchStatus(ctx, rootNode.Name, patch); err != nil {
			return reconcile.Result{
				RequeueAfter: RequeueTime,
			}, fmt.Errorf("failed to patch node resources: %v, will requeue", err)
		}
	}
	return reconcile.Result{}, nil
}

func mergeMap(origin, new map[string]string) map[string]string {
	if origin == nil {
		return new
	}
	if new != nil {
		for k, v := range origin {
			if _, exists := new[k]; !exists {
				new[k] = v
			}
		}
	}
	delete(new, utils.LabelNodeRoleControlPlane)
	delete(new, utils.LabelNodeRoleOldControlPlane)
	delete(new, utils.LabelNodeRoleNode)
	return new
}
