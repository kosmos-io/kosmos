package utils

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

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

	assert.True(t, IsRootCluster(cluster), "Expected the cluster to be identified as a root cluster")

	// Test for non-root cluster
	nonRootCluster := &kosmosv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "non-root-cluster",
			Annotations: map[string]string{},
		},
	}
	assert.False(t, IsRootCluster(nonRootCluster), "Expected the cluster to not be identified as a root cluster")
}

func TestGetAddress(t *testing.T) {
	// Setup fake Kubernetes client
	clientset := fake.NewSimpleClientset()

	// Create mock node addresses
	originAddresses := []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "192.168.1.1"},
		{Type: corev1.NodeExternalIP, Address: "203.0.113.5"},
	}

	// Set environment variables for PREFERRED-ADDRESS-TYPE and LEAF_NODE_IP
	os.Setenv("PREFERRED-ADDRESS-TYPE", string(corev1.NodeExternalDNS))
	os.Setenv("LEAF_NODE_IP", "192.168.99.99")

	// Call GetAddress
	addresses, err := GetAddress(context.TODO(), clientset, originAddresses)

	assert.NoError(t, err, "Expected no error from GetAddress")
	assert.Len(t, addresses, 3, "Expected 3 addresses")
	assert.Equal(t, "192.168.99.99", addresses[0].Address, "First address should be the LEAF_NODE_IP")
}

func TestSortAddress(t *testing.T) {
	// Setup fake Kubernetes client
	clientset := fake.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.1.1.1"},
				},
			},
		},
	)

	// Create mock node addresses (both IPv4 and IPv6)
	originAddresses := []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "192.168.1.1"},
		{Type: corev1.NodeInternalIP, Address: "2001:db8::1"},
		{Type: corev1.NodeExternalIP, Address: "203.0.113.5"},
	}

	// Test for IPv4 priority
	os.Setenv("PREFERRED-ADDRESS-TYPE", string(corev1.NodeInternalDNS))
	os.Setenv("LEAF_NODE_IP", "192.168.99.99")

	addresses, err := SortAddress(context.TODO(), clientset, originAddresses)

	assert.NoError(t, err, "Expected no error from SortAddress")
	assert.Len(t, addresses, 3, "Expected 3 sorted addresses")
	assert.Equal(t, "192.168.1.1", addresses[0].Address, "IPv4 should come first")
	assert.Equal(t, "2001:db8::1", addresses[1].Address, "IPv6 should come second")

	// Test for IPv6 priority
	err = clientset.CoreV1().Nodes().Delete(context.TODO(), "test-node", metav1.DeleteOptions{})
	if err != nil {
		t.Errorf("Error deleting node: %v", err)
	}
	_, err = clientset.CoreV1().Nodes().Create(context.TODO(), &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "2001:db8::1"},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("Error deleting node: %v", err)
	}

	addresses, err = SortAddress(context.TODO(), clientset, originAddresses)

	assert.NoError(t, err, "Expected no error from SortAddress")
	assert.Equal(t, "2001:db8::1", addresses[0].Address, "IPv6 should come first")
	assert.Equal(t, "192.168.1.1", addresses[1].Address, "IPv4 should come second")
}
