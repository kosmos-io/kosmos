package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func TestClassificationHandler(t *testing.T) {
	// 创建一个假的 Kubernetes 客户端
	rootClientset := fake.NewSimpleClientset()
	leafClientset := fake.NewSimpleClientset()

	clusterParty := &kosmosv1alpha1.Cluster{
		Spec: kosmosv1alpha1.ClusterSpec{
			ClusterTreeOptions: &kosmosv1alpha1.ClusterTreeOptions{
				Enable: true,
				LeafModels: []kosmosv1alpha1.LeafModel{
					{
						NodeSelector: kosmosv1alpha1.NodeSelector{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "test"},
							},
						},
					},
				},
			},
		},
	}

	handlerParty := NewLeafModelHandler(clusterParty, rootClientset, leafClientset)

	modeParty := handlerParty.GetLeafMode()
	if modeParty != Party {
		t.Errorf("expected modeParty to remain 3, got %v", modeParty)
	}

	if !clusterParty.Spec.ClusterTreeOptions.Enable {
		t.Errorf("expected Enable to remain true, got %v", clusterParty.Spec.ClusterTreeOptions.Enable)
	}

	clusterALL := &kosmosv1alpha1.Cluster{
		Spec: kosmosv1alpha1.ClusterSpec{
			ClusterTreeOptions: &kosmosv1alpha1.ClusterTreeOptions{
				Enable: false,
			},
		},
	}

	handlerALL := NewLeafModelHandler(clusterALL, rootClientset, leafClientset)

	modeALL := handlerALL.GetLeafMode()
	if modeALL != ALL {
		t.Errorf("expected GetLeafNodes to remain 1, got %v", modeALL)
	}

	if !clusterParty.Spec.ClusterTreeOptions.Enable {
		t.Errorf("expected Enable to remain false, got %v", clusterParty.Spec.ClusterTreeOptions.Enable)
	}
}
