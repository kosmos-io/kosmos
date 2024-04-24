package vcnodecontroller

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
	vcrnodepoolcontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.nodepool.controller"
)

func (r *NodeController) joinNodeToHost(ctx context.Context, nodeInfo vcrnodepoolcontroller.NodeItem) error {
	masterNodeIP := os.Getenv("EXECTOR_HOST_MASTER_NODE_IP")
	hostPort := ""
	if len(masterNodeIP) == 0 {
		return fmt.Errorf("get master node ip from env failed")
	}
	hostExectorHelper := exector.NewExectorHelper(masterNodeIP, hostPort)
	joinCmdStrCmd := &exector.CMDExector{
		Cmd: "kubeadm token create --print-join-command",
	}
	// step(1/3) get join cmd
	ret := hostExectorHelper.DoExector(ctx.Done(), joinCmdStrCmd)
	if ret.Status != exector.SUCCESS {
		return fmt.Errorf("get host join cmd on node %s failed: %s", nodeInfo.Name, ret.String())
	}
	joinCmdStr, err := getJoinCmdStr(ret.LastLog)
	if err != nil {
		return err
	}

	exectHelper := exector.NewExectorHelper(nodeInfo.Address, "")
	// step(2/3) remove node from old cluster
	resetCmd := &exector.CMDExector{
		Cmd: "sh kubelet_node_helper.sh unjoin",
	}

	ret = exectHelper.DoExector(ctx.Done(), resetCmd)
	if ret.Status != exector.SUCCESS {
		return fmt.Errorf("reset node %s failed: %s", nodeInfo.Name, ret.String())
	}

	// step(3/3) add node to host-cluster
	joinCmd := &exector.CMDExector{
		Cmd: joinCmdStr,
	}

	ret = exectHelper.DoExector(ctx.Done(), joinCmd)
	if ret.Status != exector.SUCCESS {
		return fmt.Errorf("exec join cmd on node %s failed: %s, join cmd: %s", nodeInfo.Name, ret.String(), joinCmdStr)
	}

	return nil
}

func (r *NodeController) unjoinNode(ctx context.Context, nodeInfos []vcrnodepoolcontroller.NodeItem, k8sClient kubernetes.Interface) error {
	// delete node from cluster
	for _, nodeInfo := range nodeInfos {
		// remove node from cluster
		klog.V(4).Infof("start remove node from cluster, node name: %s", nodeInfo.Name)
		err := k8sClient.CoreV1().Nodes().Delete(ctx, nodeInfo.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("remove node from cluster failed, node name: %s", nodeInfo.Name)
			return fmt.Errorf("%s, %s", nodeInfo.Name, err)
		}
		klog.V(4).Infof("remove node from cluster successed, node name: %s", nodeInfo.Name)

		// TODO: reset kubeadm-flags.env

		// TODO： move to node pool controller，  add node to host cluster
		if err := r.joinNodeToHost(ctx, nodeInfo); err != nil {
			klog.Errorf("join node %s to host cluster failed: %s", nodeInfo.Name, err)
			return err
		}
		// update nodepool status
		if err := r.UpdateNodePoolState(ctx, nodeInfo.Name, NodePoolStateFree); err != nil {
			return err
		}
	}
	return nil
}
