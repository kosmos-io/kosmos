package utils

import (
	"context"
	"fmt"
	"os"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	RootClusterAnnotationKey   = "kosmos.io/cluster-role"
	RootClusterAnnotationValue = "root"
)

// IsRootCluster checks if a cluster is root cluster
func IsRootCluster(cluster *kosmosv1alpha1.Cluster) bool {
	annotations := cluster.GetAnnotations()
	if val, ok := annotations[RootClusterAnnotationKey]; ok {
		return val == RootClusterAnnotationValue
	}
	return false
}

func GetAddress(ctx context.Context, rootClient kubernetes.Interface, originAddress []corev1.NodeAddress) ([]corev1.NodeAddress, error) {
	preferredAddressType := corev1.NodeAddressType(os.Getenv("PREFERRED-ADDRESS-TYPE"))

	if len(preferredAddressType) == 0 {
		preferredAddressType = corev1.NodeInternalDNS
	}

	prefixAddress := []corev1.NodeAddress{
		{Type: preferredAddressType, Address: os.Getenv("LEAF_NODE_IP")},
	}

	address, err := SortAddress(ctx, rootClient, originAddress)

	if err != nil {
		return nil, err
	}

	return append(prefixAddress, address...), nil
}

func SortAddress(ctx context.Context, rootClient kubernetes.Interface, originAddress []corev1.NodeAddress) ([]corev1.NodeAddress, error) {
	rootnodes, err := rootClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("create node failed, cannot get node from root cluster, err: %v", err)
	}

	if len(rootnodes.Items) == 0 {
		return nil, fmt.Errorf("create node failed, cannot get node from root cluster, len of leafnodes is 0")
	}

	isIPv4First := true
	for _, addr := range rootnodes.Items[0].Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			if utils.IsIPv6(addr.Address) {
				isIPv4First = false
			}
			break
		}
	}

	address := []corev1.NodeAddress{}
	otherAddress := []corev1.NodeAddress{}

	for _, addr := range originAddress {
		if addr.Type == corev1.NodeInternalIP {
			address = append(address, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: addr.Address})
		} else {
			otherAddress = append(otherAddress, addr)
		}
	}

	sort.Slice(address, func(i, j int) bool {
		if isIPv4First {
			if !utils.IsIPv6(address[i].Address) && utils.IsIPv6(address[j].Address) {
				return true
			}
			if utils.IsIPv6(address[i].Address) && !utils.IsIPv6(address[j].Address) {
				return false
			}
			return true
		} else {
			if !utils.IsIPv6(address[i].Address) && utils.IsIPv6(address[j].Address) {
				return false
			}
			if utils.IsIPv6(address[i].Address) && !utils.IsIPv6(address[j].Address) {
				return true
			}
			return true
		}
	})

	return append(address, otherAddress...), nil
}
