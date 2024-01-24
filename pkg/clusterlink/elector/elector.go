package elector

import (
	"context"
	"net"
	"os"
	"sort"

	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/controllers/node"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/role"
)

type Elector struct {
	nodeName           string
	clusterName        string
	controlPanelClient versioned.Interface
}

func NewElector(controlPanelClient versioned.Interface) *Elector {
	return &Elector{
		nodeName:           os.Getenv(utils.EnvNodeName),
		clusterName:        os.Getenv(utils.EnvClusterName),
		controlPanelClient: controlPanelClient,
	}
}

func (e *Elector) EnsureGateWayRole() error {
	clusterNodes, err := e.controlPanelClient.KosmosV1alpha1().ClusterNodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	cluster, err := e.controlPanelClient.KosmosV1alpha1().Clusters().Get(context.TODO(), e.clusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if len(cluster.Spec.ClusterLinkOptions.NodeElasticIPMap) > 0 {
		// TODO: it's not a good way, there is problem when one cluster has EIP and other clusters do not have,
		var readyNodes = make([]string, 0, 5)
		currentNodeName := os.Getenv(utils.EnvNodeName)
		elasticIPMap := cluster.Spec.ClusterLinkOptions.NodeElasticIPMap
		isCurrentNodeWithEIP := false
		needReelect := true

		for nodeName := range elasticIPMap {
			if nodeName == currentNodeName {
				isCurrentNodeWithEIP = true
				break
			}
		}
		// check all node's elasticIP is valid
		for nodeName := range elasticIPMap {
			if net.ParseIP(elasticIPMap[nodeName]) == nil {
				klog.Errorf("elasticIP %s is invalid", elasticIPMap[nodeName])
				continue
			}
			clusternode, err := e.controlPanelClient.KosmosV1alpha1().ClusterNodes().Get(context.TODO(),
				node.ClusterNodeName(e.clusterName, nodeName), metav1.GetOptions{})
			if err != nil {
				klog.Errorf("node %s is invalid: %v", nodeName, err)
				continue
			}
			if len(clusternode.Status.NodeStatus) > 0 &&
				clusternode.Status.NodeStatus == string(apicorev1.NodeReady) {
				// if some node with elasticIP is valid, don't need reelect
				if clusternode.IsGateway() {
					needReelect = false
					e.nodeName = clusternode.Spec.NodeName
					break
				}
				// put ready nodes with elasticIP into readyNodes slice
				readyNodes = append(readyNodes, nodeName)
			}
		}

		if needReelect {
			if !isCurrentNodeWithEIP && len(readyNodes) > 0 {
				// TODO: now choose first one, find a better way
				sort.Strings(readyNodes)
				e.nodeName = readyNodes[0]
			} else {
				e.nodeName = os.Getenv(utils.EnvNodeName)
			}
		}
	} else {
		e.nodeName = os.Getenv(utils.EnvNodeName)
	}

	modifyNodes := e.genModifyNode(clusterNodes.Items)
	if len(modifyNodes) > 0 {
		klog.Infof("%d node need modify", len(modifyNodes))
	}
	for i := range modifyNodes {
		node := modifyNodes[i]
		_, err := e.controlPanelClient.KosmosV1alpha1().ClusterNodes().Update(context.TODO(), &node, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("update clusterNode %s with role %v err: %v", node.Name, node.Spec.Roles, err)
			return err
		}
		klog.Infof("update clusterNode %s with role %v success", node.Name, node.Spec.Roles)
	}
	return nil
}

func (e *Elector) genModifyNode(clusterNodes []v1alpha1.ClusterNode) []v1alpha1.ClusterNode {
	var modifyNodes = make([]v1alpha1.ClusterNode, 0, 5)
	for i := range clusterNodes {
		clusterNode := clusterNodes[i]
		isGateWay := clusterNode.IsGateway()
		isSameCluster := clusterNode.Spec.ClusterName == e.clusterName
		isNewGwNode := clusterNode.Spec.NodeName == e.nodeName
		if isSameCluster {
			if !isNewGwNode && isGateWay {
				role.RemoveRole(&clusterNode, v1alpha1.RoleGateway)
				modifyNodes = append(modifyNodes, clusterNode)
			} else if isNewGwNode && !isGateWay {
				role.AddRole(&clusterNode, v1alpha1.RoleGateway)
				modifyNodes = append(modifyNodes, clusterNode)
			}
		}
	}
	return modifyNodes
}
