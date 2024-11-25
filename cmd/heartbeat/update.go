package heartbeat

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// updatePodStatus 更新 Pod 的状态信息
func updatePodStatus(clientset *kubernetes.Clientset, namespace, podName, condition string) error {
	ctx := context.TODO()

	// 获取目标 Pod
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	// 更新 Pod 的状态条件
	pod.Status.Conditions = append(pod.Status.Conditions, v1.PodCondition{
		Type:   v1.PodReady,
		Status: v1.ConditionStatus(condition),
		Reason: "NodeAgentCheck",
		Message: fmt.Sprintf("node-agent service is %s",
			condition),
		LastProbeTime: metav1.Time{Time: time.Now()},
	})

	_, err = clientset.CoreV1().Pods(namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod status: %v", err)
	}
	return nil
}

func main() {
	// 初始化 Kubernetes 客户端
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// 假设 namespace 和 podName 是已知的
	namespace := "default"
	podName := "example-pod"

	// 定时检测服务状态并上报
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status := "False"
			if checkNodeAgentStatus() {
				status = "True"
			}
			err := updatePodStatus(clientset, namespace, podName, status)
			if err != nil {
				fmt.Printf("failed to update pod status: %v\n", err)
			} else {
				fmt.Println("Updated pod status successfully")
			}
		}
	}
}
