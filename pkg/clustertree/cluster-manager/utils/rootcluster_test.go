package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func TestIsRootCluster(t *testing.T) {
	// Create a mock cluster with the root annotation
	cluster := &kosmosv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			Annotations: map[string]string{
				RootClusterAnnotationKey: RootClusterAnnotationValue,
			},
		},
	}

	if !IsRootCluster(cluster) {
		t.Errorf("expected IsRootCluster to remain true, got %v", "false")
	}

	// Test for non-root cluster
	nonRootCluster := &kosmosv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "non-root-cluster",
			Annotations: map[string]string{},
		},
	}

	if IsRootCluster(nonRootCluster) {
		t.Errorf("Expected the cluster to not be identified as a root cluster")
	}
}
