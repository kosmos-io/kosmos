package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestLeafResourceManager_AddLeafResource(t *testing.T) {
	lrm := GetGlobalLeafResourceManager()

	cluster := &kosmosv1alpha1.Cluster{
		Spec: kosmosv1alpha1.ClusterSpec{
			ClusterTreeOptions: &kosmosv1alpha1.ClusterTreeOptions{
				LeafModels: []kosmosv1alpha1.LeafModel{
					{
						NodeSelector: kosmosv1alpha1.NodeSelector{
							NodeName: "node-1",
						},
					},
				},
			},
		},
	}

	nodes := []*corev1.Node{
		{
			Spec: corev1.NodeSpec{
				ProviderID: "node-1",
			},
		},
	}

	leafResource := &LeafResource{
		Cluster:   cluster,
		Namespace: "default",
	}

	lrm.AddLeafResource(leafResource, nodes)

	result, err := lrm.GetLeafResource(cluster.Name)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "node-1", result.Nodes[0].NodeName)
	assert.Equal(t, Node, result.Nodes[0].LeafMode)
}

func TestLeafResourceManager_RemoveLeafResource(t *testing.T) {
	lrm := GetGlobalLeafResourceManager()

	clusterName := "test-cluster"
	cluster := &kosmosv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
	}

	leafResource := &LeafResource{
		Cluster:   cluster,
		Namespace: "default",
	}

	lrm.AddLeafResource(leafResource, nil)
	lrm.RemoveLeafResource(clusterName)

	_, err := lrm.GetLeafResource(clusterName)
	assert.Error(t, err)
}

func TestLeafResourceManager_GetLeafResourceByNodeName(t *testing.T) {
	lrm := GetGlobalLeafResourceManager()

	cluster := &kosmosv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
	}

	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
		},
	}

	leafResource := &LeafResource{
		Cluster:   cluster,
		Namespace: "default",
	}

	lrm.AddLeafResource(leafResource, nodes)

	result, err := lrm.GetLeafResourceByNodeName("node-1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-cluster", result.Cluster.Name)
}

func TestLeafResourceManager_HasNode(t *testing.T) {
	lrm := GetGlobalLeafResourceManager()

	cluster := &kosmosv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
	}

	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
		},
	}

	leafResource := &LeafResource{
		Cluster:   cluster,
		Namespace: "default",
	}

	lrm.AddLeafResource(leafResource, nodes)

	assert.True(t, lrm.HasNode("node-1"))
	assert.False(t, lrm.HasNode("node-2"))
}

func TestLeafResourceManager_ListNodesAndClusters(t *testing.T) {
	lrm := GetGlobalLeafResourceManager()

	cluster1 := &kosmosv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-1",
		},
	}
	cluster2 := &kosmosv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-2",
		},
	}

	nodes1 := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
		},
	}
	nodes2 := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
		},
	}

	leafResource1 := &LeafResource{
		Cluster:   cluster1,
		Namespace: "default",
	}
	leafResource2 := &LeafResource{
		Cluster:   cluster2,
		Namespace: "default",
	}

	lrm.AddLeafResource(leafResource1, nodes1)
	lrm.AddLeafResource(leafResource2, nodes2)

	nodes := lrm.ListNodes()
	clusters := lrm.ListClusters()

	assert.Equal(t, 2, len(nodes))
	assert.Contains(t, nodes, "node-1")
	assert.Contains(t, nodes, "node-2")

	assert.Equal(t, 2, len(clusters))
	assert.Contains(t, clusters, "cluster-1")
	assert.Contains(t, clusters, "cluster-2")
}
