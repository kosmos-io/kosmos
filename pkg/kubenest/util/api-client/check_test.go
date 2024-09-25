package util

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake" // 使用 kubernetes 的 fake 客户端
)

// TestVirtualClusterChecker_WaitForSomePods 测试 WaitForSomePods 方法
func TestVirtualClusterChecker_WaitForSomePods(t *testing.T) {
	// 创建一个 fake 客户端，包含一些 Pod
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	})

	checker := NewVirtualClusterChecker(client, 10*time.Second)
	err := checker.WaitForSomePods("app=test", "default", 2)
	assert.NoError(t, err)
}

// TestVirtualClusterChecker_WaitForSomePods_Error 测试 WaitForSomePods 方法失败情况
func TestVirtualClusterChecker_WaitForSomePods_Error(t *testing.T) {
	// 创建一个 fake 客户端，包含一些 Pod
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			},
		},
	})

	checker := NewVirtualClusterChecker(client, 10*time.Second)
	err := checker.WaitForSomePods("app=test1", "default", 2)
	assert.Error(t, err)
}

// TestTryRunCommand 测试 TryRunCommand 函数
func TestTryRunCommand(t *testing.T) {
	count := 0
	f := func() error {
		count++
		if count < 3 {
			return fmt.Errorf("error")
		}
		return nil
	}

	err := TryRunCommand(f, 3)
	assert.NoError(t, err)
}

func TestTryRunCommand_Error(t *testing.T) {
	f := func() error {
		return fmt.Errorf("error")
	}

	err := TryRunCommand(f, 1)
	assert.Error(t, err)
}
