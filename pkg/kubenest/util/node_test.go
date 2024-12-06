package util

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientTesting "k8s.io/client-go/testing"
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
		//{
		//	name:          "missing client",
		//	nodeName:      fakeNodeName,
		//	client:        nil,
		//	node:          fakeNode,
		//	drainWaitSecs: 30,
		//	isHostCluster: true,
		//	wantErr:       true,
		//},
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
					return true, nil, apierrors.NewNotFound(v1.Resource("nodes"), "non-existent-node")
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

func TestGetAPIServerNodes(t *testing.T) {
	namespace := "test-namespace"

	t.Run("Successfully Get API Server Nodes", func(t *testing.T) {
		apiServerPod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apiserver-pod-1",
				Namespace: namespace,
				Labels:    map[string]string{"virtualCluster-app": "apiserver"},
			},
			Spec: v1.PodSpec{
				NodeName: "node1",
			},
		}
		apiServerNode := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{
					{Type: v1.NodeInternalIP, Address: "192.168.1.10"},
				},
			},
		}

		client := fake.NewSimpleClientset(apiServerPod, apiServerNode)

		nodes, err := GetAPIServerNodes(client, namespace)
		assert.NoError(t, err, "Should successfully get API server nodes")
		assert.Len(t, nodes.Items, 1, "Expected exactly one node")
		assert.Equal(t, "node1", nodes.Items[0].Name, "Node name should match")
	})

	t.Run("No API Server Pods Found", func(t *testing.T) {
		client := fake.NewSimpleClientset()

		nodes, err := GetAPIServerNodes(client, namespace)
		assert.Error(t, err, "Should fail when no API server pods are found")
		assert.Contains(t, err.Error(), "no API server pods found", "Error message should match")
		assert.Nil(t, nodes, "Nodes should be nil when no API server pods are found")
	})

	t.Run("Error Listing API Server Pods", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("list", "pods", func(action clientTesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("mock error: failed to list pods")
		})

		nodes, err := GetAPIServerNodes(client, namespace)
		assert.Error(t, err, "Should fail when listing pods returns an error")
		assert.Contains(t, err.Error(), "failed to list kube-apiserver pods", "Error message should match")
		assert.Nil(t, nodes, "Nodes should be nil when pod listing fails")
	})

	t.Run("Error Fetching Node Information", func(t *testing.T) {
		apiServerPod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apiserver-pod-1",
				Namespace: namespace,
				Labels:    map[string]string{"virtualCluster-app": "apiserver"},
			},
			Spec: v1.PodSpec{
				NodeName: "node1",
			},
		}

		client := fake.NewSimpleClientset(apiServerPod)
		client.PrependReactor("get", "nodes", func(action clientTesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("mock error: failed to get node")
		})

		nodes, err := GetAPIServerNodes(client, namespace)
		assert.Error(t, err, "Should fail when fetching node information returns an error")
		assert.Contains(t, err.Error(), "failed to get node", "Error message should match")
		assert.Nil(t, nodes, "Nodes should be nil when node fetching fails")
	})

	t.Run("Pod Exists but Node Not Found", func(t *testing.T) {
		apiServerPod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apiserver-pod-1",
				Namespace: namespace,
				Labels:    map[string]string{"virtualCluster-app": "apiserver"},
			},
			Spec: v1.PodSpec{
				NodeName: "node1",
			},
		}

		client := fake.NewSimpleClientset(apiServerPod)

		nodes, err := GetAPIServerNodes(client, namespace)
		assert.Error(t, err, "Should fail when node does not exist")
		assert.Contains(t, err.Error(), "failed to get node", "Error message should match")
		assert.Nil(t, nodes, "Nodes should be nil when node is not found")
	})
}
