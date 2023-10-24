package util

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// WaitPodReady wait pod ready.
func WaitPodReady(c kubernetes.Interface, namespace, selector string, timeout int) error {
	// Wait 3 second
	time.Sleep(3 * time.Second)
	pods, err := c.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods in %s with selector %s", namespace, selector)
	}

	for _, pod := range pods.Items {
		if err = waitPodReady(c, namespace, pod.Name, time.Duration(timeout)*time.Second); err != nil {
			return err
		}
	}

	return nil
}

// waitPodReady  Poll up to timeout seconds for pod to enter running state.
// Returns an error if the pod never enters the running state.
func waitPodReady(c kubernetes.Interface, namespaces, podName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, isPodReady(c, namespaces, podName))
}

func podStatus(pod *corev1.Pod) string {
	for _, value := range pod.Status.ContainerStatuses {
		if pod.Status.Phase == corev1.PodRunning {
			if value.State.Waiting != nil {
				return value.State.Waiting.Reason
			}
			if value.State.Waiting == nil {
				return string(corev1.PodRunning)
			}
			return "Error"
		}
		if pod.ObjectMeta.DeletionTimestamp != nil {
			return "Terminating"
		}
	}
	return pod.Status.ContainerStatuses[0].State.Waiting.Reason
}

func isPodReady(c kubernetes.Interface, n, p string) wait.ConditionFunc {
	return func() (done bool, err error) {
		pod, err := c.CoreV1().Pods(n).Get(context.TODO(), p, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if pod.Status.Phase == corev1.PodPending && len(pod.Status.ContainerStatuses) == 0 {
			klog.Warningf("Pod: %s not ready. status: %v", pod.Name, corev1.PodPending)
			return false, nil
		}

		for _, v := range pod.Status.Conditions {
			switch v.Type {
			case corev1.PodReady:
				if v.Status == corev1.ConditionTrue {
					klog.Infof("pod: %s is ready. status: %v", pod.Name, podStatus(pod))
					return true, nil
				}
				klog.Warningf("Pod: %s not ready. status: %v", pod.Name, podStatus(pod))
				return false, nil
			default:
				continue
			}
		}
		return false, err
	}
}

// WaitDeploymentReady  wait deployment ready or timeout.
func WaitDeploymentReady(c kubernetes.Interface, d *appsv1.Deployment, timeoutSeconds int) error {
	var lastErr error

	pollError := wait.PollImmediate(time.Second, time.Duration(timeoutSeconds)*time.Second, func() (bool, error) {
		deploy, err := c.AppsV1().Deployments(d.GetNamespace()).Get(context.TODO(), d.GetName(), metav1.GetOptions{})
		if err != nil {
			lastErr = err
			return false, nil
		}
		if deploy.Generation != deploy.Status.ObservedGeneration {
			lastErr = fmt.Errorf("current generation %d, observed generation %d",
				deploy.Generation, deploy.Status.ObservedGeneration)
			return false, nil
		}
		if (deploy.Spec.Replicas != nil) && (deploy.Status.UpdatedReplicas < *d.Spec.Replicas) {
			lastErr = fmt.Errorf("the number of pods targeted by the deployment (%d pods) is different "+
				"from the number of pods targeted by the deployment that have the desired template spec (%d pods)",
				*deploy.Spec.Replicas, deploy.Status.UpdatedReplicas)
			return false, nil
		}
		if deploy.Status.AvailableReplicas < deploy.Status.UpdatedReplicas {
			lastErr = fmt.Errorf("expected %d replicas, got %d available replicas",
				deploy.Status.UpdatedReplicas, deploy.Status.AvailableReplicas)
			return false, nil
		}
		return true, nil
	})
	if pollError != nil {
		return fmt.Errorf("wait for Deployment(%s/%s) ready: %v: %v", d.GetNamespace(), d.GetName(), pollError, lastErr)
	}

	return nil
}

// MapToString  labels to string.
func MapToString(labels map[string]string) string {
	v := new(bytes.Buffer)
	for key, value := range labels {
		_, err := fmt.Fprintf(v, "%s=%s,", key, value)
		if err != nil {
			klog.Errorf("map to string error: %s", err)
		}
	}
	return strings.TrimRight(v.String(), ",")
}

func CheckInstall(modules string) {
	fmt.Printf(`
--------------------------------------------------------------------------------------
 █████   ████    ███████     █████████  ██████   ██████    ███████     █████████
░░███   ███░   ███░░░░░███  ███░░░░░███░░██████ ██████   ███░░░░░███  ███░░░░░███
 ░███  ███    ███     ░░███░███    ░░░  ░███░█████░███  ███     ░░███░███    ░░░
 ░███████    ░███      ░███░░█████████  ░███░░███ ░███ ░███      ░███░░█████████
 ░███░░███   ░███      ░███ ░░░░░░░░███ ░███ ░░░  ░███ ░███      ░███ ░░░░░░░░███
 ░███ ░░███  ░░███     ███  ███    ░███ ░███      ░███ ░░███     ███  ███    ░███
 █████ ░░████ ░░░███████░  ░░█████████  █████     █████ ░░░███████░  ░░█████████
░░░░░   ░░░░    ░░░░░░░     ░░░░░░░░░  ░░░░░     ░░░░░    ░░░░░░░     ░░░░░░░░░
---------------------------------------------------------------------------------------
Kosmos has been installed successfully. The module %[1]s is installed.

`, modules)
}
