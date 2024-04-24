package vcnodecontroller

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
)

// kubeadm join
func getJoinCmdStr(log string) (string, error) {
	strs := strings.Split(log, "kubeadm join")
	if len(strs) != 2 {
		return "", fmt.Errorf("get join cmd str failed")
	}
	return fmt.Sprintf("kubeadm join %s", strs[1]), nil
}

func isNodeReady(conditions []v1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *NodeController) WaitNodeReady(ctx context.Context, globalNode v1alpha1.GlobalNode, k8sClient kubernetes.Interface) error {
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second) // total waiting time
	defer cancel()

	isReady := false

	wait.UntilWithContext(waitCtx, func(ctx context.Context) {
		node, err := k8sClient.CoreV1().Nodes().Get(waitCtx, globalNode.Name, metav1.GetOptions{})
		if err == nil {
			if isNodeReady(node.Status.Conditions) {
				klog.V(4).Infof("node %s is ready", globalNode.Name)
				isReady = true
				cancel()
			} else {
				klog.V(4).Infof("node %s is not ready, status: %s", globalNode.Name, node.Status.Phase)
			}
		} else {
			klog.V(4).Infof("get node %s failed: %s", globalNode.Name, err)
		}
	}, 10*time.Second) // Interval time

	if isReady {
		return nil
	}

	return fmt.Errorf("node %s is not ready", globalNode.Name)
}

func (r *NodeController) joinNode(ctx context.Context, globalNodes []v1alpha1.GlobalNode, virtualCluster v1alpha1.VirtualCluster, k8sClient kubernetes.Interface) error {
	if len(globalNodes) == 0 {
		return nil
	}

	clusterDNS := "127.0.0.1"
	dnssvc, err := k8sClient.CoreV1().Services((KubeDNSNS)).Get(ctx, KubeDNSName, metav1.GetOptions{})
	if err != nil {
		// TODO: wait dns
		// return fmt.Errorf("get kube-dns service failed: %s", err)
		klog.Errorf("get kube-dns service failed: %s", err)
	} else {
		clusterDNS = dnssvc.Spec.ClusterIP
	}

	for _, globalNode := range globalNodes {
		// add node to new cluster
		exectHelper := exector.NewExectorHelper(globalNode.Spec.NodeIP, "")

		// check
		checkCmd := &exector.CMDExector{
			Cmd: "sh kubelet_node_helper.sh check",
		}
		ret := exectHelper.DoExector(ctx.Done(), checkCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("check node %s failed: %s", globalNode.Name, ret.String())
		}

		// step(1/5) reset node
		resetCmd := &exector.CMDExector{
			Cmd: "sh kubelet_node_helper.sh unjoin",
		}
		ret = exectHelper.DoExector(ctx.Done(), resetCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("reset node %s failed: %s", globalNode.Name, ret.String())
		}
		// step(2/5) scp ca of virtualcluster
		nn := types.NamespacedName{
			Namespace: virtualCluster.Namespace,
			Name:      fmt.Sprintf("%s-cert", virtualCluster.Name),
		}
		targetCert := &v1.Secret{}
		if err := r.Get(ctx, nn, targetCert); err != nil {
			return fmt.Errorf("get target cert %s failed: %s", nn, err)
		}

		cacrt := targetCert.Data["ca.crt"]
		scpCrtCmd := &exector.SCPExector{
			DstFilePath: ExectorTmpPath,
			DstFileName: "ca.crt",
			SrcByte:     cacrt,
		}
		ret = exectHelper.DoExector(ctx.Done(), scpCrtCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("scp ca.crt to node %s failed: %s", globalNode.Name, ret.String())
		}

		// step(3/5) scp kubeconfig of virtualcluster
		kubeconfig, err := base64.StdEncoding.DecodeString(virtualCluster.Spec.Kubeconfig)
		if err != nil {
			return fmt.Errorf("decode target kubeconfig %s failed: %s", nn, err)
		}

		scpKCCmd := &exector.SCPExector{
			DstFilePath: ExectorTmpPath,
			DstFileName: "kubelet.conf",
			SrcByte:     kubeconfig,
		}
		ret = exectHelper.DoExector(ctx.Done(), scpKCCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("scp kubeconfig to node %s failed: %s", globalNode.Name, ret.String())
		}

		// step(4/5) scp kubelet config
		kubeletConfigPath := os.Getenv("EXECTOR_KUBELET_CONFIG_PATH")
		if len(kubeletConfigPath) == 0 {
			kubeletConfigPath = "/bin/config.yaml"
		}
		scpKubeletConfigCmd := &exector.SCPExector{
			DstFilePath: ExectorTmpPath,
			DstFileName: "config.yaml",
			SrcFile:     kubeletConfigPath, // from configmap volumn
		}

		ret = exectHelper.DoExector(ctx.Done(), scpKubeletConfigCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("scp kubelet config to node %s failed: %s", globalNode.Name, ret.String())
		}

		// step(5/5) join node
		joinCmd := &exector.CMDExector{
			Cmd: fmt.Sprintf("sh kubelet_node_helper.sh join %s", clusterDNS),
		}
		ret = exectHelper.DoExector(ctx.Done(), joinCmd)
		if ret.Status != exector.SUCCESS {
			return fmt.Errorf("join node %s failed: %s", globalNode.Name, ret.String())
		}

		// wait node ready
		if err := r.WaitNodeReady(ctx, globalNode, k8sClient); err != nil {
			klog.Errorf("wait node %s ready failed: %s", globalNode.Name, err)
			return err
		}

		// TODO: maybe change kubeadm-flags.env
		// add label
		node, err := k8sClient.CoreV1().Nodes().Get(ctx, globalNode.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get node %s failed: %s", globalNode.Name, err)
		}

		updateNode := node.DeepCopy()
		for k, v := range globalNode.Spec.Labels {
			node.Labels[k] = v
		}

		if _, err := k8sClient.CoreV1().Nodes().Update(ctx, updateNode, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("add label to node %s failed: %s", globalNode.Name, err)
		}

		updateGlobalNode := globalNode.DeepCopy()
		updateGlobalNode.Spec.State = v1alpha1.NodeInUse
		if err := r.Client.Update(context.TODO(), updateGlobalNode); err != nil {
			klog.Errorf("update global node %s state failed: %s", globalNode.Name, err)
			return err
		}
	}
	return nil
}
