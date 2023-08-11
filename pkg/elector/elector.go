package elector

import (
	"context"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/clusterlink/pkg/utils"
	"github.com/kosmos.io/clusterlink/pkg/utils/role"
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
	clusterNodes, err := e.controlPanelClient.ClusterlinkV1alpha1().ClusterNodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	_, err = e.controlPanelClient.ClusterlinkV1alpha1().Clusters().Get(context.TODO(), e.clusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	modifyNodes := e.genModifyNode(clusterNodes.Items)
	klog.Infof("%d node need modify", len(modifyNodes))
	for _, node := range modifyNodes {
		_, err := e.controlPanelClient.ClusterlinkV1alpha1().ClusterNodes().Update(context.TODO(), &node, metav1.UpdateOptions{})
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
	for _, clusterNode := range clusterNodes {
		isGateWay := clusterNode.IsGateway()
		isSameCluster := clusterNode.Spec.ClusterName == e.clusterName
		isCurrentNode := clusterNode.Spec.NodeName == e.nodeName
		if isSameCluster {
			if !isCurrentNode && isGateWay {
				role.RemoveRole(&clusterNode, v1alpha1.RoleGateway)
				modifyNodes = append(modifyNodes, clusterNode)
			} else if isCurrentNode && !isGateWay {
				role.AddRole(&clusterNode, v1alpha1.RoleGateway)
				modifyNodes = append(modifyNodes, clusterNode)
			}
		}
	}
	return modifyNodes
}
