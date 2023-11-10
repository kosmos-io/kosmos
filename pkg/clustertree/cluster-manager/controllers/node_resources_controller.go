package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
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

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/register"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	NodeResourcesControllerName = "node-resources-controller"
	RequeueTime                 = 10 * time.Second
)

type NodeResourcesController struct {
	Leaf          client.Client
	Root          client.Client
	RootClientset kubernetes.Interface

	Node          *corev1.Node
	EventRecorder record.EventRecorder
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

	nodes := corev1.NodeList{}
	if err := c.Leaf.List(ctx, &nodes); err != nil {
		return controllerruntime.Result{}, err
	}

	pods := corev1.PodList{}
	if err := c.Leaf.List(ctx, &pods); err != nil {
		return controllerruntime.Result{}, err
	}

	clusterResources := utils.CalculateClusterResources(&nodes, &pods)

	node := &corev1.Node{}
	err := c.Root.Get(ctx, types.NamespacedName{Name: c.Node.Name}, node)
	if err != nil {
		return reconcile.Result{
			RequeueAfter: RequeueTime,
		}, fmt.Errorf("cannot get node while update node resources %s, err: %v", c.Node.Name, err)
	}

	clone := node.DeepCopy()
	clone.Status.Allocatable = clusterResources
	clone.Status.Capacity = clusterResources
	clone.Status.Conditions = utils.NodeConditions()

	patch, err := utils.CreateMergePatch(node, clone)
	if err != nil {
		return reconcile.Result{}, err
	}

	if _, err := c.RootClientset.CoreV1().Nodes().PatchStatus(ctx, c.Node.Name, patch); err != nil {
		return reconcile.Result{
			RequeueAfter: RequeueTime,
		}, fmt.Errorf("failed to patch node resources: %v, will requeue", err)
	}

	return reconcile.Result{}, nil
}

func init() {
	register.RegisterLeafController(NodeResourcesControllerName, func(cl *register.LeafControllerOptions) error {
		nodeResourcesController := NodeResourcesController{
			Leaf:          cl.Mgr.GetClient(),
			Root:          cl.RootClient,
			RootClientset: cl.RootClientSet,
			Node:          cl.Node,
		}
		if err := nodeResourcesController.SetupWithManager(cl.Mgr); err != nil {
			return fmt.Errorf("error starting %s: %v", NodeResourcesControllerName, err)
		}
		return nil
	})
}
