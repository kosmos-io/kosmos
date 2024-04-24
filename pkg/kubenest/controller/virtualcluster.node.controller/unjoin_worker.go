package vcnodecontroller

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
)

func (r *NodeController) joinNodeToHost(ctx context.Context, globalNode v1alpha1.GlobalNode) error {
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
		return fmt.Errorf("get host join cmd on node %s failed: %s", globalNode.Name, ret.String())
	}
	joinCmdStr, err := getJoinCmdStr(ret.LastLog)
	if err != nil {
		return err
	}

	exectHelper := exector.NewExectorHelper(globalNode.Spec.NodeIP, "")
	// step(2/3) remove node from old cluster
	resetCmd := &exector.CMDExector{
		Cmd: "sh kubelet_node_helper.sh unjoin",
	}

	ret = exectHelper.DoExector(ctx.Done(), resetCmd)
	if ret.Status != exector.SUCCESS {
		return fmt.Errorf("reset node %s failed: %s", globalNode.Name, ret.String())
	}

	// step(3/3) add node to host-cluster
	joinCmd := &exector.CMDExector{
		Cmd: joinCmdStr,
	}

	ret = exectHelper.DoExector(ctx.Done(), joinCmd)
	if ret.Status != exector.SUCCESS {
		return fmt.Errorf("exec join cmd on node %s failed: %s, join cmd: %s", globalNode.Name, ret.String(), joinCmdStr)
	}

	return nil
}

func (r *NodeController) unjoinNode(ctx context.Context, GlobalNodes []v1alpha1.GlobalNode, k8sClient kubernetes.Interface) error {
	// delete node from cluster
	for _, globalNode := range GlobalNodes {
		// remove node from cluster
		klog.V(4).Infof("start remove node from cluster, node name: %s", globalNode.Name)
		err := k8sClient.CoreV1().Nodes().Delete(ctx, globalNode.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("remove node from cluster failed, node name: %s", globalNode.Name)
			return fmt.Errorf("%s, %s", globalNode.Name, err)
		}
		klog.V(4).Infof("remove node from cluster successed, node name: %s", globalNode.Name)

		// TODO: reset kubeadm-flags.env

		// TODO： move to node pool controller，  add node to host cluster
		if err := r.joinNodeToHost(ctx, globalNode); err != nil {
			klog.Errorf("join node %s to host cluster failed: %s", globalNode.Name, err)
			return err
		}

		updateGlobalNode := globalNode.DeepCopy()
		updateGlobalNode.Spec.State = v1alpha1.NodeFreeState
		if err := r.Client.Update(context.TODO(), updateGlobalNode); err != nil {
			klog.Errorf("update global node %s state failed: %s", globalNode.Name, err)
			return err
		}
	}
	return nil
}
