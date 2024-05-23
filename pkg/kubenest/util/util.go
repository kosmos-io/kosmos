package util

import (
	"encoding/base64"
	"fmt"
	"net"

	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func FindGlobalNode(nodeName string, globalNodes []v1alpha1.GlobalNode) (*v1alpha1.GlobalNode, bool) {
	for _, globalNode := range globalNodes {
		if globalNode.Name == nodeName {
			return &globalNode, true
		}
	}
	return nil, false
}

func GenerateKubeclient(virtualCluster *v1alpha1.VirtualCluster) (kubernetes.Interface, error) {
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

func GetFirstIP(ipNetStr string) (net.IP, error) {
	_, ipNet, err := net.ParseCIDR(ipNetStr)
	if err != nil {
		fmt.Println("parse ipNetStr err:", err)
		return nil, err
	}

	firstIP := make(net.IP, len(ipNet.IP))
	copy(firstIP, ipNet.IP)
	for i := range firstIP {
		firstIP[i] |= ipNet.Mask[i]
	}

	return firstIP, nil
}
