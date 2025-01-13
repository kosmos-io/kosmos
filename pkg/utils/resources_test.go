// nolint:dupl
package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// TestGetNodesTotalResources tests the GetNodesTotalResources function
func TestGetNodesTotalResources(t *testing.T) {
	tests := []struct {
		name     string
		nodes    *corev1.NodeList
		expected corev1.ResourceList
		notNodes map[string]corev1.Node
	}{
		{
			name: "No nodes",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{},
			},
			expected: corev1.ResourceList{},
		},
		{
			name: "Single unschedulable node",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						Spec: corev1.NodeSpec{
							Unschedulable: true,
						},
						Status: corev1.NodeStatus{
							Allocatable: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
							},
						},
					},
				},
			},
			expected: corev1.ResourceList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNodesTotalResources(tt.nodes, tt.notNodes)
			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("expected %s for %s, got %v", expectedValue.String(), key.String(), result[key])
				}
			}
		})
	}
}

// TestSubResourceList tests the SubResourceList function.
func TestSubResourceList(t *testing.T) {
	tests := []struct {
		name     string
		base     corev1.ResourceList
		list     corev1.ResourceList
		expected corev1.ResourceList
	}{
		{
			name: "Subtracting resources not present in base",
			base: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			list: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("4"), // CPU 不变
			},
		},
		{
			name:     "Base and list are empty",
			base:     corev1.ResourceList{},
			list:     corev1.ResourceList{},
			expected: corev1.ResourceList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SubResourceList(tt.base, tt.list)
			for key, expectedValue := range tt.expected {
				if tt.base[key] != expectedValue {
					t.Errorf("expected %s for %s, got %v", expectedValue.String(), key.String(), tt.base[key])
				}
			}
		})
	}
}

// TestGetPodsTotalRequestsAndLimits tests the GetPodsTotalRequestsAndLimits function.
// 测试用例说明：
// 1. **No pods**: 测试没有 Pod 的情况，期望返回空请求和限制。
// 2. **Single running pod with requests and limits**: 测试单个运行中的 Pod，具有请求和限制。
// 3. **Running pod with no requests and limits**: 测试单个运行中的 Pod，没有请求和限制。
// 4. **Non-running pod**: 测试非运行中的 Pod，期望返回空请求和限制。
// 5. **Virtual pod**: 测试虚拟 Pod，期望返回空请求和限制。
func TestGetPodsTotalRequestsAndLimits(t *testing.T) {
	tests := []struct {
		name           string
		podList        *corev1.PodList
		expectedReqs   corev1.ResourceList
		expectedLimits corev1.ResourceList
		notNodes       map[string]corev1.Node
	}{
		{
			name: "No pods",
			podList: &corev1.PodList{
				Items: []corev1.Pod{},
			},
			expectedReqs:   corev1.ResourceList{},
			expectedLimits: corev1.ResourceList{},
		},
		{
			name: "Single running pod with requests and limits",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("1"),
											corev1.ResourceMemory: resource.MustParse("1Gi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("2"),
											corev1.ResourceMemory: resource.MustParse("2Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedReqs: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			expectedLimits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		},
		{
			name: "Running pod with no requests and limits",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Resources: corev1.ResourceRequirements{},
								},
							},
						},
					},
				},
			},
			expectedReqs:   corev1.ResourceList{},
			expectedLimits: corev1.ResourceList{},
		},
		{
			name: "Non-running pod",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodPending,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU: resource.MustParse("1"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceMemory: resource.MustParse("1Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedReqs:   corev1.ResourceList{},
			expectedLimits: corev1.ResourceList{},
		},
		{
			name: "Virtual pod",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU: resource.MustParse("1"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceMemory: resource.MustParse("1Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedReqs:   corev1.ResourceList{},
			expectedLimits: corev1.ResourceList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, limits := GetPodsTotalRequestsAndLimits(tt.podList, tt.notNodes)
			for key, expectedValue := range tt.expectedReqs {
				if reqs[key] != expectedValue {
					t.Errorf("expected %s for requests %s, got %v", expectedValue.String(), key.String(), reqs[key])
				}
			}
			for key, expectedValue := range tt.expectedLimits {
				if limits[key] != expectedValue {
					t.Errorf("expected %s for limits %s, got %v", expectedValue.String(), key.String(), limits[key])
				}
			}
		})
	}
}
