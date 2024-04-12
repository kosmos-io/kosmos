package util

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	netutils "k8s.io/utils/net"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

func GetAPIServiceIP(clientset clientset.Interface) (string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return "", fmt.Errorf("there are no nodes in cluster, err: %w", err)
	}

	var (
		masterLabel       = labels.Set{"node-role.kubernetes.io/master": ""}
		controlplaneLabel = labels.Set{"node-role.kubernetes.io/control-plane": ""}
	)
	// first, select the master node as the IP of APIServer. if there is
	// no master nodes, randomly select a worker node.
	for _, node := range nodes.Items {
		ls := labels.Set(node.GetLabels())

		if masterLabel.AsSelector().Matches(ls) || controlplaneLabel.AsSelector().Matches(ls) {
			if ip := netutils.ParseIPSloppy(node.Status.Addresses[0].Address); ip != nil {
				return ip.String(), nil
			}
		}
	}
	return nodes.Items[0].Status.Addresses[0].Address, nil
}

func GetAPIServiceClusterIp(namespace string, client clientset.Interface) (error, string) {
	serviceLists, err := client.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err, ""
	}
	if serviceLists != nil {
		for _, service := range serviceLists.Items {
			if service.Spec.Type == constants.ServiceType {
				return nil, service.Spec.ClusterIP
			}
		}
	}
	return nil, ""
}

func GetServiceClusterIp(namespace string, client clientset.Interface) (error, []string) {
	serviceLists, err := client.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err, nil
	}
	var clusterIps []string
	if serviceLists != nil {
		for _, service := range serviceLists.Items {
			if service.Spec.ClusterIP != "" {
				clusterIps = append(clusterIps, service.Spec.ClusterIP)
			}
		}
	}
	return nil, clusterIps
}

func GetEtcdServiceClusterIp(namespace string, client clientset.Interface) (error, []string) {
	serviceLists, err := client.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err, nil
	}
	var clusterIps []string
	if serviceLists != nil {
		for _, service := range serviceLists.Items {
			if service.Spec.Type == constants.EtcdServiceType && service.Spec.ClusterIP != "" {
				clusterIps = append(clusterIps, service.Spec.ClusterIP)
			}
		}
	}
	return nil, clusterIps
}
