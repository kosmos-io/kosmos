package endpointcontroller

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

type MockNodeGetter struct {
	Nodes *corev1.NodeList
	Err   error
}

func (m *MockNodeGetter) GetAPIServerNodes(_ kubernetes.Interface, _ string) (*corev1.NodeList, error) {
	return m.Nodes, m.Err
}

func TestSyncAPIServerExternalEndpoints(t *testing.T) {
	ctx := context.TODO()
	vc := &v1alpha1.VirtualCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vc",
			Namespace: "test-ns",
		},
		Status: v1alpha1.VirtualClusterStatus{
			Phase: v1alpha1.Completed,
			PortMap: map[string]int32{
				constants.APIServerPortKey: 6443,
			},
		},
	}

	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{Type: corev1.NodeInternalIP, Address: "192.168.1.1"},
					},
				},
			},
		},
	}

	endpoint := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.APIServerExternalService,
			Namespace: constants.KosmosNs,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{IP: "192.168.1.1"},
				},
				Ports: []corev1.EndpointPort{
					{Name: "https", Port: 6443, Protocol: corev1.ProtocolTCP},
				},
			},
		},
	}

	tests := []struct {
		name          string
		objects       []runtime.Object
		mockNodes     *corev1.NodeList
		mockErr       error
		wantErr       bool
		wantErrString string
		wantSubsets   []corev1.EndpointSubset
	}{
		{
			name:        "Successfully syncs external endpoints",
			objects:     []runtime.Object{},
			mockNodes:   nodes,
			wantSubsets: endpoint.Subsets,
		},
		{
			name:        "Does not update endpoint if no changes",
			objects:     []runtime.Object{endpoint},
			mockNodes:   nodes,
			wantSubsets: endpoint.Subsets,
		},
		{
			name: "Updates endpoint if changes detected",
			objects: []runtime.Object{
				func() runtime.Object {
					modifiedEndpoint := endpoint.DeepCopy()
					modifiedEndpoint.Subsets[0].Addresses[0].IP = "192.168.1.2"
					return modifiedEndpoint
				}(),
			},
			mockNodes:   nodes,
			wantSubsets: endpoint.Subsets,
		},
		{
			name:          "Fails if no API server nodes are found",
			objects:       []runtime.Object{},
			mockNodes:     &corev1.NodeList{},
			wantErr:       true,
			wantErrString: "no API server nodes found in the cluster",
		},
		{
			name:    "Fails if no internal IP addresses are found",
			objects: []runtime.Object{},
			mockNodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
						Status: corev1.NodeStatus{
							Addresses: []corev1.NodeAddress{},
						},
					},
				},
			},
			wantErr:       true,
			wantErrString: "no internal IP addresses found for the API server nodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use fake clientset to simulate the Kubernetes API client (host cluster)
			fakeHostClusterClient := fake.NewSimpleClientset(tt.objects...)
			// Simulate the Virtual Cluster client by passing the same clientset
			fakeVCClient := fake.NewSimpleClientset()
			// Mock NodeGetter to return the mock nodes for Host cluster
			mockNodeGetter := &MockNodeGetter{Nodes: tt.mockNodes, Err: tt.mockErr}
			// Use fake clientset to simulate the Kubernetes API client (host cluster)
			controller := &APIServerExternalSyncController{
				KubeClient: fakeHostClusterClient,
				NodeGetter: mockNodeGetter,
			}
			// Test the controller method using the VC's client
			err := controller.SyncAPIServerExternalEndpoints(ctx, fakeVCClient, vc)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrString != "" {
					assert.Contains(t, err.Error(), tt.wantErrString)
				}
			} else {
				assert.NoError(t, err)
				if tt.wantSubsets != nil {
					createdEndpoint, err := fakeVCClient.CoreV1().Endpoints(constants.KosmosNs).Get(ctx, constants.APIServerExternalService, metav1.GetOptions{})
					assert.NoError(t, err)
					assert.True(t, reflect.DeepEqual(createdEndpoint.Subsets, tt.wantSubsets))
				}
			}
		})
	}
}
