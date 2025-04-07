package vcnodemanager

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.manager/env"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.manager/workflow"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.manager/workflow/task"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

type NodeManager struct {
	client.Client
	RootClientSet kubernetes.Interface
	Options       *v1alpha1.KubeNestConfiguration
	sem           chan struct{}
}

func NewNodeManager(client client.Client, RootClientSet kubernetes.Interface, options *v1alpha1.KubeNestConfiguration) *NodeManager {
	r := NodeManager{
		Client:        client,
		RootClientSet: RootClientSet,
		Options:       options,
		sem:           make(chan struct{}, env.GetNodeTaskMaxGoroutines()),
	}
	return &r
}

func hasItemInArray(name string, f func(string) bool) bool {
	return f(name)
}

func (r *NodeManager) compareAndTranformNodes(ctx context.Context, targetNodes []v1alpha1.NodeInfo, actualNodes []v1.Node) ([]v1alpha1.GlobalNode, []v1alpha1.GlobalNode, error) {
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

func (r *NodeManager) DoNodeTask(ctx context.Context, virtualCluster v1alpha1.VirtualCluster) error {
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

	return nil
}
func (r *NodeManager) NodeDelete(ctx context.Context, virtualCluster v1alpha1.VirtualCluster) error {
	if err := r.DoNodeClean(ctx, virtualCluster); err != nil {
		klog.Errorf("virtualcluster %s do node clean failed: %v", virtualCluster.Name, err)
		return err
	}
	return nil
}

func (r *NodeManager) NodeUpdate(ctx context.Context, virtualCluster v1alpha1.VirtualCluster) error {
	if len(virtualCluster.Spec.Kubeconfig) == 0 {
		klog.Warning("virtualcluster.spec.kubeconfig is nil, wait virtualcluster control-plane ready.")
		return fmt.Errorf("virtualcluster.spec.kubeconfig is nil, wait virtualcluster control-plane ready. %s", virtualCluster.GetName())
	}

	if err := r.DoNodeTask(ctx, virtualCluster); err != nil {
		klog.Errorf("virtualcluster %s do node task failed: %v", virtualCluster.Name, err)
		return fmt.Errorf("update virtualcluster %s status error: %v", virtualCluster.GetName(), err)
	}
	return nil
}

func (r *NodeManager) DoNodeClean(ctx context.Context, virtualCluster v1alpha1.VirtualCluster) error {
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

func (r *NodeManager) cleanGlobalNode(ctx context.Context, nodeInfos []v1alpha1.GlobalNode, virtualCluster v1alpha1.VirtualCluster, _ kubernetes.Interface) error {
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

func (r *NodeManager) joinNode(ctx context.Context, nodeInfos []v1alpha1.GlobalNode, virtualCluster v1alpha1.VirtualCluster, k8sClient kubernetes.Interface) error {
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

func (r *NodeManager) unjoinNode(ctx context.Context, nodeInfos []v1alpha1.GlobalNode, virtualCluster v1alpha1.VirtualCluster, k8sClient kubernetes.Interface) error {
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

func (r *NodeManager) BatchProcessNodes(nodeInfos []v1alpha1.GlobalNode, f func(v1alpha1.GlobalNode) error) error {
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
