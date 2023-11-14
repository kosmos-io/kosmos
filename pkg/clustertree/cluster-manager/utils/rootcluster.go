package utils

import (
	"os"

	corev1 "k8s.io/api/core/v1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
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

func GetAddress() []corev1.NodeAddress {
	address := []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: os.Getenv("KNODE_POD_IP")},
	}
	return address
}
