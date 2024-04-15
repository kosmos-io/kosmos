package vcnodecontroller

import (
	"context"
	"encoding/base64"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	vcrnodepoolcontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.nodepool.controller"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type NodeController struct {
	client.Client
	EventRecorder record.EventRecorder
}

// TODO: status
func (r *NodeController) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	skipEvent := func(_ client.Object) bool {
		return true
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.NodeControllerName).
		WithOptions(controller.Options{}).
		For(&v1alpha1.VirtualCluster{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return skipEvent(createEvent.Object)
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return skipEvent(updateEvent.ObjectNew)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return skipEvent(deleteEvent.Object)
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return skipEvent(genericEvent.Object)
			},
		})).
		Complete(r)
}

func (c *NodeController) GenerateKubeclient(virtualCluster *v1alpha1.VirtualCluster) (kubernetes.Interface, error) {
	if len(virtualCluster.Spec.Kubeconfig) == 0 {
		return nil, fmt.Errorf("virtualcluster %s kubeconfig is empty", virtualCluster.Name)
	}
	kubeconfigStream, err := base64.StdEncoding.DecodeString(virtualCluster.Spec.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("virtualcluster %s decode target kubernetes kubeconfig %s err: %v", virtualCluster.Name, virtualCluster.Spec.Kubeconfig, err)
	}

	config, err := utils.NewConfigFromBytes(kubeconfigStream)
	if err != nil {
		return nil, fmt.Errorf("generate kubernetes config failed: %s", err)
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("generate K8s basic client failed: %v", err)
	}

	return k8sClient, nil
}

func hasItemInArray(name string, f func(string) bool) bool {
	return f(name)
}

func compareAndTranformNodes(targetNodes []v1alpha1.NodeInfo, actualNodes []v1.Node, nodePools map[string]vcrnodepoolcontroller.NodeItem) ([]vcrnodepoolcontroller.NodeItem, []vcrnodepoolcontroller.NodeItem, error) {
	unjoinNodes := []vcrnodepoolcontroller.NodeItem{}
	joinNodes := []vcrnodepoolcontroller.NodeItem{}

	// cacheMap := map[string]string{}
	for _, targetNode := range targetNodes {
		has := hasItemInArray(targetNode.NodeName, func(name string) bool {
			for _, actualNode := range actualNodes {
				if actualNode.Name == name {
					return true
				}
			}
			return false
		})

		if !has {
			nodePool, ok := nodePools[targetNode.NodeName]
			if !ok {
				return nil, nil, fmt.Errorf("node %s not found in node pool", targetNode.NodeName)
			}
			joinNodes = append(joinNodes, nodePool)
		}
	}

	for _, actualNode := range actualNodes {
		has := hasItemInArray(actualNode.Name, func(name string) bool {
			for _, targetNode := range targetNodes {
				if targetNode.NodeName == name {
					return true
				}
			}
			return false
		})

		if !has {
			nodePool, ok := nodePools[actualNode.Name]
			if !ok {
				return nil, nil, fmt.Errorf("node %s not found in node pool", actualNode.Name)
			}

			unjoinNodes = append(unjoinNodes, nodePool)
		}
	}

	return unjoinNodes, joinNodes, nil
}

func (r *NodeController) GetNodePool(ctx context.Context) (map[string]vcrnodepoolcontroller.NodeItem, error) {
	nodePool := v1.ConfigMap{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: NodePoolCMName, Namespace: NodePoolCMNS}, &nodePool); err != nil {
		return nil, fmt.Errorf("get node-pool failed: %v", err)
	}

	nodePools, err := vcrnodepoolcontroller.ConvertYamlToNodeItem(nodePool.Data[NodePoolCMKeyName])
	if err != nil {
		return nil, fmt.Errorf("convert node-pool failed: %v", err)
	}

	return nodePools, nil
}

func (r *NodeController) UpdateVirtualClusterStatus(ctx context.Context, virtualCluster v1alpha1.VirtualCluster, status v1alpha1.Phase, reason string) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		targetObj := v1alpha1.VirtualCluster{}
		if err := r.Get(ctx, types.NamespacedName{Name: virtualCluster.Name, Namespace: virtualCluster.Namespace}, &targetObj); err != nil {
			return err
		}
		updateVirtualCluster := targetObj.DeepCopy()
		updateVirtualCluster.Status.Phase = status
		updateVirtualCluster.Status.Reason = reason
		updateTime := metav1.Now()
		updateVirtualCluster.Status.UpdateTime = &updateTime

		if err := r.Update(ctx, updateVirtualCluster); err != nil {
			return err
		}
		return nil
	})

	if retryErr != nil {
		return fmt.Errorf("update virtualcluster %s status failed: %s", virtualCluster.Name, retryErr)
	}

	return nil
}

func (r *NodeController) DoNodeTask(ctx context.Context, virtualCluster v1alpha1.VirtualCluster) error {
	k8sClient, err := r.GenerateKubeclient(&virtualCluster)
	if err != nil {
		return fmt.Errorf("virtualcluster %s crd kubernetes client failed: %v", virtualCluster.Name, err)
	}

	nodes, err := k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("virtualcluster %s get virtual-cluster nodes list failed: %v", virtualCluster.Name, err)
	}

	nodePools, err := r.GetNodePool(ctx)
	if err != nil {
		return err
	}

	// compare cr and actual nodes in k8s
	unjoinNodes, joinNodes, err := compareAndTranformNodes(virtualCluster.Spec.PromoteResources.NodeInfos, nodes.Items, nodePools)
	if err != nil {
		return fmt.Errorf("compare cr and actual nodes failed, virtual-cluster-name: %v, err: %s", virtualCluster.Name, err)
	}

	if len(unjoinNodes) > 0 || len(joinNodes) > 0 {
		if virtualCluster.Status.Phase == v1alpha1.Initialized {
			if err := r.UpdateVirtualClusterStatus(ctx, virtualCluster, v1alpha1.Initialized, "node init"); err != nil {
				return err
			}
		} else {
			if err := r.UpdateVirtualClusterStatus(ctx, virtualCluster, v1alpha1.Updating, "node task"); err != nil {
				return err
			}
		}
	}
	if len(unjoinNodes) > 0 {
		// unjoin node
		if err := r.unjoinNode(ctx, unjoinNodes, k8sClient); err != nil {
			return fmt.Errorf("virtualcluster %s unjoin node failed: %v", virtualCluster.Name, err)
		}
	}
	if len(joinNodes) > 0 {
		// join node
		if err := r.joinNode(ctx, joinNodes, virtualCluster, k8sClient); err != nil {
			return fmt.Errorf("virtualcluster %s join node failed: %v", virtualCluster.Name, err)
		}
	}

	if len(unjoinNodes) > 0 || len(joinNodes) > 0 {
		if err := r.UpdateVirtualClusterStatus(ctx, virtualCluster, v1alpha1.AllNodeReady, "node ready"); err != nil {
			return err
		}
	}
	return nil
}

func (r *NodeController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ virtual-cluster-node-controller start to reconcile %s ============", request.NamespacedName)
	defer klog.V(4).Infof("============ virtual-cluster-node-controller finish to reconcile %s ============", request.NamespacedName)

	// check virtual cluster nodes
	var virtualCluster v1alpha1.VirtualCluster
	if err := r.Get(ctx, request.NamespacedName, &virtualCluster); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("virtual-cluster-node-controller: can not found %s", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		klog.Errorf("get clusternode %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if !virtualCluster.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	if virtualCluster.Status.Phase == v1alpha1.Preparing {
		klog.V(4).Infof("virtualcluster wait cluster ready, cluster name: %s", virtualCluster.Name)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if err := r.DoNodeTask(ctx, virtualCluster); err != nil {
		klog.Errorf("virtualcluster %s do node task failed: %v", virtualCluster.Name, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}
