package pod

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

type MockLeafResourceManager struct {
	mock.Mock
}

func (m *MockLeafResourceManager) AddLeafResource(lr *leafUtils.LeafResource, nodes []*corev1.Node) {
	m.Called(lr, nodes)
}

func (m *MockLeafResourceManager) RemoveLeafResource(clusterName string) {
	m.Called(clusterName)
}

func (m *MockLeafResourceManager) GetLeafResource(clusterName string) (*leafUtils.LeafResource, error) {
	args := m.Called(clusterName)
	return args.Get(0).(*leafUtils.LeafResource), args.Error(1)
}

func (m *MockLeafResourceManager) GetLeafResourceByNodeName(nodeName string) (*leafUtils.LeafResource, error) {
	args := m.Called(nodeName)
	return args.Get(0).(*leafUtils.LeafResource), args.Error(1)
}

func (m *MockLeafResourceManager) HasCluster(clusterName string) bool {
	args := m.Called(clusterName)
	return args.Bool(0)
}

func (m *MockLeafResourceManager) HasNode(nodeName string) bool {
	args := m.Called(nodeName)
	return args.Bool(0)
}

func (m *MockLeafResourceManager) ListNodes() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockLeafResourceManager) ListClusters() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockLeafResourceManager) GetClusterNode(nodeName string) *leafUtils.ClusterNode {
	args := m.Called(nodeName)
	return args.Get(0).(*leafUtils.ClusterNode)
}

type MockLeafClientResourceManager struct {
	mock.Mock
}

func (m *MockLeafClientResourceManager) AddLeafClientResource(lcr *leafUtils.LeafClientResource, cluster *kosmosv1alpha1.Cluster) {
	m.Called(lcr, cluster)
}

func (m *MockLeafClientResourceManager) RemoveLeafClientResource(actualClusterName string) {
	m.Called(actualClusterName)
}

func (m *MockLeafClientResourceManager) GetLeafResource(actualClusterName string) (*leafUtils.LeafClientResource, error) {
	args := m.Called(actualClusterName)
	return args.Get(0).(*leafUtils.LeafClientResource), args.Error(1)
}

func (m *MockLeafClientResourceManager) ListActualClusters() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// TestReconcile verifies different scenarios in Reconcile function
func TestReconcile(t *testing.T) {
	// Set up scheme
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// // Initialize mocks
	mockLeafManager := new(MockLeafResourceManager)
	mockLeafClientManager := new(MockLeafClientResourceManager)

	// Test data: root pod
	rootPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			Labels: map[string]string{
				"kosmos-io/pod": "true",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "kosmos-leaf",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	leafPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			Labels: map[string]string{
				"kosmos-io/pod": "true",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "leaf-worker",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	// Fake Kubernetes client with root pod
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rootPod).Build()

	// Mock GlobalLeafManager behavior
	mockLeafManager.On("HasNode", "kosmos-leaf").Return(true)
	mockLeafManager.On("GetLeafResourceByNodeName", "kosmos-leaf").Return(&leafUtils.LeafResource{
		Cluster: &kosmosv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
		},
	}, nil)

	// Mock GlobalLeafClientManager behavior
	mockLeafClientManager.On("GetLeafResource", "test-cluster").Return(&leafUtils.LeafClientResource{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(leafPod).Build(),
	}, nil)

	// Initialize reconciler
	reconciler := &RootPodSyncReconciler{
		RootClient:              fakeClient,
		GlobalLeafManager:       mockLeafManager,
		GlobalLeafClientManager: mockLeafClientManager,
	}

	// Reconcile request
	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
	}

	// Execute Reconcile
	result, err := reconciler.Reconcile(context.TODO(), request)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result)

	// Verify Root Pod status update
	updatedPod := &corev1.Pod{}
	err = fakeClient.Get(context.TODO(), request.NamespacedName, updatedPod)
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(updatedPod.Status, leafPod.Status))
}
