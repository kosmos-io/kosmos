package util

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"

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

func GetFirstIP(ipNetStrs string) ([]net.IP, error) {
	ipNetStrArray := strings.Split(ipNetStrs, ",")
	if len(ipNetStrArray) > 2 {
		return nil, fmt.Errorf("getFirstIP failed, ipstring is too long: %s", ipNetStrs)
	}

	var ips []net.IP
	for _, ipNetStr := range ipNetStrArray {
		ip, ipNet, err := net.ParseCIDR(ipNetStr)
		if err != nil {
			return nil, fmt.Errorf("parse ipNetStr failed: %s", err)
		}

		networkIP := ip.Mask(ipNet.Mask)

		// IPv4
		if ip.To4() != nil {
			firstIP := make(net.IP, len(networkIP))
			copy(firstIP, networkIP)
			firstIP[len(firstIP)-1]++
			ips = append(ips, firstIP)
			continue
		}

		// IPv6
		firstIP := make(net.IP, len(networkIP))
		copy(firstIP, networkIP)
		for i := len(firstIP) - 1; i >= 0; i-- {
			firstIP[i]++
			if firstIP[i] != 0 {
				break
			}
		}
		ips = append(ips, firstIP)
	}
	return ips, nil
}

func IPV6First(ipNetStr string) (bool, error) {
	ipNetStrArray := strings.Split(ipNetStr, ",")
	if len(ipNetStrArray) > 2 {
		return false, fmt.Errorf("getFirstIP failed, ipstring is too long: %s", ipNetStr)
	}
	return utils.IsIPv6(ipNetStrArray[0]), nil
}
