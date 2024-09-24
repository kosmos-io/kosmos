package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func TestClassificationHandler(t *testing.T) {
	// 创建一个假的 Kubernetes 客户端
	rootClientset := fake.NewSimpleClientset()
	leafClientset := fake.NewSimpleClientset()

	cluster := &kosmosv1alpha1.Cluster{
		Spec: kosmosv1alpha1.ClusterSpec{
			ClusterTreeOptions: &kosmosv1alpha1.ClusterTreeOptions{
				Enable: true,
				LeafModels: []kosmosv1alpha1.LeafModel{
					{
						NodeSelector: kosmosv1alpha1.NodeSelector{
							LabelSelector: &v1.LabelSelector{
								MatchLabels: map[string]string{"app": "test"},
							},
						},
					},
				},
			},
		},
	}

	handler := NewLeafModelHandler(cluster, rootClientset, leafClientset)

	// 测试 GetLeafNodes 方法
	t.Run("GetLeafNodes", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name: "test-node",
			},
			Spec: corev1.NodeSpec{},
		}

		_, err := handler.GetLeafNodes(context.TODO(), node, kosmosv1alpha1.NodeSelector{})
		if err != nil {
			t.Fatalf("GetLeafNodes error: %v", err)
		}
		leafNodes, err := handler.GetLeafNodes(context.TODO(), node, cluster.Spec.ClusterTreeOptions.LeafModels[0].NodeSelector)
		if err != nil {
			t.Fatalf("GetLeafNodes error: %v", err)
		}
		assert.Equal(t, 1, len(leafNodes.Items))
		assert.Equal(t, "test-node", leafNodes.Items[0].Name)
	})

	// 测试 GetLeafPods 方法
	t.Run("GetLeafPods", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name: "test-node",
			},
			Spec: corev1.NodeSpec{},
		}

		_, err := handler.GetLeafPods(context.TODO(), node, kosmosv1alpha1.NodeSelector{})
		if err != nil {
			t.Fatalf("GetLeafNodes error: %v", err)
		}
		pods, err := handler.GetLeafPods(context.TODO(), node, cluster.Spec.ClusterTreeOptions.LeafModels[0].NodeSelector)
		if err != nil {
			t.Fatalf("GetLeafNodes error: %v", err)
		}
		assert.Equal(t, 1, len(pods.Items))
		assert.Equal(t, "test-pod", pods.Items[0].Name)
	})

	// 测试 UpdateRootNodeStatus 方法
	t.Run("UpdateRootNodeStatus", func(t *testing.T) {
		rootNode := &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name: "root-node",
			},
		}

		err := handler.UpdateRootNodeStatus(context.TODO(), []*corev1.Node{rootNode}, map[string]kosmosv1alpha1.NodeSelector{})
		if err != nil {
			t.Fatalf("GetLeafNodes error: %v", err)
		}
		err = handler.UpdateRootNodeStatus(context.TODO(), []*corev1.Node{rootNode}, map[string]kosmosv1alpha1.NodeSelector{
			"root-node": cluster.Spec.ClusterTreeOptions.LeafModels[0].NodeSelector,
		})
		if err != nil {
			t.Fatalf("GetLeafNodes error: %v", err)
		}
		assert.NoError(t, err)
	})
}
