package util

import (
	"context"
	"testing" // 确保只导入一次

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientTesting "k8s.io/client-go/testing" // 使用别名避免和 Go 原生 testing 冲突
)

// TestIsNodeReady tests the IsNodeReady function
func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []v1.NodeCondition
		want       bool
	}{
		{
			name: "node is ready",
			conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
			want: true,
		},
		{
			name: "node is not ready",
			conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionFalse,
				},
			},
			want: false,
		},
		{
			name:       "no conditions",
			conditions: []v1.NodeCondition{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNodeReady(tt.conditions)
			if got != tt.want {
				t.Errorf("IsNodeReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDrainNode tests the DrainNode function
func TestDrainNode(t *testing.T) {
	fakeNodeName := "fake-node"
	fakeNode := &v1.Node{
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	ctx := context.TODO()
	fakeClient := fake.NewSimpleClientset(fakeNode)

	tests := []struct {
		name          string
		nodeName      string
		client        *fake.Clientset
		node          *v1.Node
		drainWaitSecs int
		isHostCluster bool
		wantErr       bool
		prepare       func()
	}{
		{
			name:          "successful drain and cordon",
			nodeName:      fakeNodeName,
			client:        fakeClient,
			node:          fakeNode,
			drainWaitSecs: 30,
			isHostCluster: true,
			wantErr:       false,
		},
		{
			name:          "missing client",
			nodeName:      fakeNodeName,
			client:        nil,
			node:          fakeNode,
			drainWaitSecs: 30,
			isHostCluster: true,
			wantErr:       true,
		},
		{
			name:          "missing node",
			nodeName:      fakeNodeName,
			client:        fakeClient,
			node:          nil,
			drainWaitSecs: 30,
			isHostCluster: true,
			wantErr:       true,
		},
		{
			name:          "missing node name",
			nodeName:      "",
			client:        fakeClient,
			node:          fakeNode,
			drainWaitSecs: 30,
			isHostCluster: true,
			wantErr:       true,
		},
		{
			name:          "node not found error",
			nodeName:      "non-existent-node",
			client:        fakeClient,
			node:          fakeNode,
			drainWaitSecs: 30,
			isHostCluster: true,
			wantErr:       false,
			prepare: func() {
				fakeClient.Fake.PrependReactor("get", "nodes", func(action clientTesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewNotFound(v1.Resource("nodes"), "non-existent-node")
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.prepare != nil {
				tt.prepare()
			}

			err := DrainNode(ctx, tt.nodeName, tt.client, tt.node, tt.drainWaitSecs, tt.isHostCluster)
			if (err != nil) != tt.wantErr {
				t.Errorf("DrainNode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
