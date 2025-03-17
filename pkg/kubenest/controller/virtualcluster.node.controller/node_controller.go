package vcnodecontroller

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
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
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/env"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/workflow"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/workflow/task"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type NodeController struct {
	client.Client
	RootClientSet kubernetes.Interface
	EventRecorder record.EventRecorder
	KosmosClient  versioned.Interface
	Options       *v1alpha1.KubeNestConfiguration
	sem           chan struct{}
}

func NewNodeController(client client.Client, RootClientSet kubernetes.Interface, EventRecorder record.EventRecorder, KosmosClient versioned.Interface, options *v1alpha1.KubeNestConfiguration) *NodeController {
	r := NodeController{
		Client:        client,
		RootClientSet: RootClientSet,
		EventRecorder: EventRecorder,
		KosmosClient:  KosmosClient,
		Options:       options,
		sem:           make(chan struct{}, env.GetNodeTaskMaxGoroutines()),
	}
	return &r
}

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

func hasItemInArray(name string, f func(string) bool) bool {
	return f(name)
}

func (r *NodeController) compareAndTranformNodes(ctx context.Context, targetNodes []v1alpha1.NodeInfo, actualNodes []v1.Node) ([]v1alpha1.GlobalNode, []v1alpha1.GlobalNode, error) {
	unjoinNodes := make([]v1alpha1.GlobalNode, 0)
	joinNodes := make([]v1alpha1.GlobalNode, 0)

	globalNodes := &v1alpha1.GlobalNodeList{}
	if err := r.Client.List(ctx, globalNodes); err != nil {
		return nil, nil, fmt.Errorf("failed to list global nodes: %v", err)
	}

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
			globalNode, ok := util.FindGlobalNode(targetNode.NodeName, globalNodes.Items)
			if !ok {
				return nil, nil, fmt.Errorf("global node %s not found", targetNode.NodeName)
			}
			joinNodes = append(joinNodes, *globalNode)
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
			globalNode, ok := util.FindGlobalNode(actualNode.Name, globalNodes.Items)
			if !ok {
				return nil, nil, fmt.Errorf("global node %s not found", actualNode.Name)
			}
			unjoinNodes = append(unjoinNodes, *globalNode)
		}
	}

	return unjoinNodes, joinNodes, nil
}

func (r *NodeController) UpdateVirtualClusterStatus(ctx context.Context, virtualCluster v1alpha1.VirtualCluster, status v1alpha1.Phase, reason string) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var targetObj v1alpha1.VirtualCluster
		if err := r.Get(ctx, types.NamespacedName{Name: virtualCluster.Name, Namespace: virtualCluster.Namespace}, &targetObj); err != nil {
			klog.Warningf("get target virtualcluster %s namespace %s failed: %v", virtualCluster.Name, virtualCluster.Namespace, err)
			return err
		}
		updateVirtualCluster := targetObj.DeepCopy()
		if len(status) > 0 {
			updateVirtualCluster.Status.Phase = status
		}
		updateVirtualCluster.Status.Reason = reason
		updateTime := metav1.Now()
		updateVirtualCluster.Status.UpdateTime = &updateTime
		if _, err := r.KosmosClient.KosmosV1alpha1().VirtualClusters(updateVirtualCluster.Namespace).Update(ctx, updateVirtualCluster, metav1.UpdateOptions{}); err != nil && !apierrors.IsNotFound(err) {
			klog.Warningf("update target virtualcluster %s namespace %s failed: %v", virtualCluster.Name, virtualCluster.Namespace, err)
			return err
		}
		return nil
	})

	if retryErr != nil {
		return fmt.Errorf("update virtualcluster %s status namespace %s failed: %s", virtualCluster.Name, virtualCluster.Namespace, retryErr)
	}

	r.EventRecorder.Event(&virtualCluster, v1.EventTypeWarning, "VCStatusPending", fmt.Sprintf("Name: %s, Namespace: %s, reason: %s", virtualCluster.Name, virtualCluster.Namespace, reason))

	return nil
}

func (r *NodeController) DoNodeTask(ctx context.Context, virtualCluster v1alpha1.VirtualCluster) error {
	k8sClient, err := util.GenerateKubeclient(&virtualCluster)
	if err != nil {
		return fmt.Errorf("virtualcluster %s crd kubernetes client failed: %v", virtualCluster.Name, err)
	}

	nodes, err := k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("virtualcluster %s get virtual-cluster nodes list failed: %v", virtualCluster.Name, err)
	}

	// compare cr and actual nodes in k8s
	unjoinNodes, joinNodes, err := r.compareAndTranformNodes(ctx, virtualCluster.Spec.PromoteResources.NodeInfos, nodes.Items)
	if err != nil {
		return fmt.Errorf("compare cr and actual nodes failed, virtual-cluster-name: %v, err: %s", virtualCluster.Name, err)
	}

	if len(unjoinNodes) > 0 || len(joinNodes) > 0 {
		if virtualCluster.Status.Phase != v1alpha1.Initialized && virtualCluster.Status.Phase != v1alpha1.Deleting {
			if err := r.UpdateVirtualClusterStatus(ctx, virtualCluster, v1alpha1.Updating, "node task"); err != nil {
				return err
			}
		}
	}
	if len(unjoinNodes) > 0 {
		// unjoin node
		if err := r.unjoinNode(ctx, unjoinNodes, virtualCluster, k8sClient); err != nil {
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
		newStatus := v1alpha1.AllNodeReady
		if virtualCluster.Status.Phase == v1alpha1.Deleting {
			newStatus = v1alpha1.AllNodeDeleted
		}
		if err := r.UpdateVirtualClusterStatus(ctx, virtualCluster, newStatus, "node ready"); err != nil {
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
		if err := r.UpdateVirtualClusterStatus(ctx, virtualCluster, v1alpha1.Pending, err.Error()); err != nil {
			klog.Errorf("update virtualcluster %s status error: %v", request.NamespacedName, err)
		}
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	if !virtualCluster.GetDeletionTimestamp().IsZero() && virtualCluster.Status.Phase != v1alpha1.Deleting {
		klog.V(4).Info("virtualcluster %s is deleting, skip node controller", virtualCluster.Name)
		return reconcile.Result{}, nil
	}

	if !virtualCluster.GetDeletionTimestamp().IsZero() && len(virtualCluster.Spec.Kubeconfig) == 0 {
		if err := r.DoNodeClean(ctx, virtualCluster); err != nil {
			klog.Errorf("virtualcluster %s do node clean failed: %v", virtualCluster.Name, err)
			if err := r.UpdateVirtualClusterStatus(ctx, virtualCluster, v1alpha1.Pending, err.Error()); err != nil {
				klog.Errorf("update virtualcluster %s status error: %v", request.NamespacedName, err)
			}
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
		return reconcile.Result{}, nil
	}

	if virtualCluster.Status.Phase == v1alpha1.Preparing {
		klog.V(4).Infof("virtualcluster wait cluster ready, cluster name: %s", virtualCluster.Name)
		return reconcile.Result{}, nil
	}

	if virtualCluster.Status.Phase == v1alpha1.Pending {
		klog.V(4).Infof("virtualcluster is pending, cluster name: %s", virtualCluster.Name)
		return reconcile.Result{}, nil
	}

	if virtualCluster.Status.Phase == v1alpha1.Completed {
		klog.V(4).Infof("virtualcluster is completed, cluster name: %s", virtualCluster.Name)
		return reconcile.Result{}, nil
	}

	if len(virtualCluster.Spec.Kubeconfig) == 0 {
		klog.Warning("virtualcluster.spec.kubeconfig is nil, wait virtualcluster control-plane ready.")
		return reconcile.Result{}, nil
	}

	if err := r.DoNodeTask(ctx, virtualCluster); err != nil {
		klog.Errorf("virtualcluster %s do node task failed: %v", virtualCluster.Name, err)
		if err := r.UpdateVirtualClusterStatus(ctx, virtualCluster, v1alpha1.Pending, err.Error()); err != nil {
			klog.Errorf("update virtualcluster %s status error: %v", request.NamespacedName, err)
		}
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}

	return reconcile.Result{}, nil
}

func (r *NodeController) DoNodeClean(ctx context.Context, virtualCluster v1alpha1.VirtualCluster) error {
	targetNodes := virtualCluster.Spec.PromoteResources.NodeInfos
	globalNodes := &v1alpha1.GlobalNodeList{}

	if err := r.Client.List(ctx, globalNodes); err != nil {
		return fmt.Errorf("failed to list global nodes: %v", err)
	}

	cleanNodeInfos := []v1alpha1.GlobalNode{}

	for _, targetNode := range targetNodes {
		globalNode, ok := util.FindGlobalNode(targetNode.NodeName, globalNodes.Items)
		if !ok {
			return fmt.Errorf("global node %s not found", targetNode.NodeName)
		}
		cleanNodeInfos = append(cleanNodeInfos, *globalNode)
	}

	return r.cleanGlobalNode(ctx, cleanNodeInfos, virtualCluster, nil)
}

func (r *NodeController) cleanGlobalNode(ctx context.Context, nodeInfos []v1alpha1.GlobalNode, virtualCluster v1alpha1.VirtualCluster, _ kubernetes.Interface) error {
	return r.BatchProcessNodes(nodeInfos, func(nodeInfo v1alpha1.GlobalNode) error {
		return workflow.NewCleanNodeWorkFlow().RunTask(ctx, task.TaskOpt{
			NodeInfo:       nodeInfo,
			VirtualCluster: virtualCluster,
			HostClient:     r.Client,
			HostK8sClient:  r.RootClientSet,
			Opt:            r.Options,
		})
	})
}

func (r *NodeController) joinNode(ctx context.Context, nodeInfos []v1alpha1.GlobalNode, virtualCluster v1alpha1.VirtualCluster, k8sClient kubernetes.Interface) error {
	if len(nodeInfos) == 0 {
		return nil
	}

	clusterDNS := ""
	dnssvc, err := k8sClient.CoreV1().Services(constants.SystemNs).Get(ctx, constants.KubeDNSSVCName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get kube-dns service failed: %s", err)
	}
	clusterDNS = dnssvc.Spec.ClusterIP

	return r.BatchProcessNodes(nodeInfos, func(nodeInfo v1alpha1.GlobalNode) error {
		return workflow.NewJoinWorkFlow().RunTask(ctx, task.TaskOpt{
			NodeInfo:         nodeInfo,
			VirtualCluster:   virtualCluster,
			KubeDNSAddress:   clusterDNS,
			HostClient:       r.Client,
			HostK8sClient:    r.RootClientSet,
			VirtualK8sClient: k8sClient,
			Opt:              r.Options,
		})
	})
}

func (r *NodeController) unjoinNode(ctx context.Context, nodeInfos []v1alpha1.GlobalNode, virtualCluster v1alpha1.VirtualCluster, k8sClient kubernetes.Interface) error {
	return r.BatchProcessNodes(nodeInfos, func(nodeInfo v1alpha1.GlobalNode) error {
		return workflow.NewUnjoinWorkFlow().RunTask(ctx, task.TaskOpt{
			NodeInfo:         nodeInfo,
			VirtualCluster:   virtualCluster,
			HostClient:       r.Client,
			HostK8sClient:    r.RootClientSet,
			VirtualK8sClient: k8sClient,
			Opt:              r.Options,
		})
	})
}

func (r *NodeController) BatchProcessNodes(nodeInfos []v1alpha1.GlobalNode, f func(v1alpha1.GlobalNode) error) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(nodeInfos))

	for _, nodeInfo := range nodeInfos {
		wg.Add(1)
		r.sem <- struct{}{}
		go func(nodeInfo v1alpha1.GlobalNode) {
			defer wg.Done()
			defer func() { <-r.sem }()
			if err := f(nodeInfo); err != nil {
				errChan <- fmt.Errorf("[%s] batchprocessnodes failed: %s", nodeInfo.Name, err)
			}
		}(nodeInfo)
	}

	wg.Wait()
	close(errChan)

	var taskErr error
	for err := range errChan {
		if err != nil {
			if taskErr == nil {
				taskErr = err
			} else {
				taskErr = errors.Wrap(err, taskErr.Error())
			}
		}
	}

	if taskErr != nil {
		return taskErr
	}

	return nil
}
