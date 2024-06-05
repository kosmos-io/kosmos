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
	ip, ipNet, err := net.ParseCIDR(ipNetStr)
	if err != nil {
		return nil, fmt.Errorf("parse ipNetStr failed: %s", err)
	}

	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("only support ipv4")
	}

	networkIP := ip.Mask(ipNet.Mask)

	firstIP := make(net.IP, len(networkIP))
	copy(firstIP, networkIP)
	firstIP[3]++

	return firstIP, nil
}

func MapContains(big map[string]string, small map[string]string) bool {
	for k, v := range small {
		if bigV, ok := big[k]; !ok || bigV != v {
			return false
		}
	}
	return true
}
