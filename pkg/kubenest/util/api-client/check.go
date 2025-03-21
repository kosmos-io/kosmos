package util

import (
	"context"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

const DefaultRetryCount = 10

type Checker interface {
	WaitForAPI() error
	WaitForSomePods(label, namespace string, podNum int32) error
}

type VirtualClusterChecker struct {
	client  clientset.Interface
	timeout time.Duration
}

func NewVirtualClusterChecker(client clientset.Interface, timeout time.Duration) Checker {
	return &VirtualClusterChecker{
		client:  client,
		timeout: timeout,
	}
}

func (v *VirtualClusterChecker) WaitForSomePods(label, namespace string, podNum int32) error {
	return wait.PollImmediate(constants.APIServerCallRetryInterval, v.timeout, func() (bool, error) {
		listOpts := metav1.ListOptions{LabelSelector: label}
		pods, err := v.client.CoreV1().Pods(namespace).List(context.TODO(), listOpts)
		if err != nil {
			return false, nil
		}

		if len(pods.Items) == 0 {
			return false, nil
		}

		var expected int32
		for _, pod := range pods.Items {
			if isPodRunning(pod) {
				expected++
			}
		}
		return expected >= podNum, nil
	})
}
func (v *VirtualClusterChecker) WaitForAPI() error {
	return wait.PollImmediate(constants.APIServerCallRetryInterval, v.timeout, func() (bool, error) {
		healthStatus := 0
		v.client.Discovery().RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			return false, nil
		}

		return true, nil
	})
}

func TryRunCommand(f func() error, failureThreshold int) error {
	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   2, // double the timeout for every failure
		Steps:    failureThreshold,
	}
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := f()
		if err != nil {
			// Retry until the timeout
			return false, nil
		}
		// The last f() call was a success, return cleanly
		return true, nil
	})
}

func isPodRunning(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
