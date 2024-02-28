package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func NewPod(namespace, name, schedulerName string, labels map[string]string, node string) *corev1.Pod {
	port := corev1.ContainerPort{
		Name:          name,
		ContainerPort: 80,
	}
	container := corev1.Container{
		Name:            name,
		Image:           "registry.paas/cmss/nginx:1.14.2",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports:           []corev1.ContainerPort{port},
	}

	if len(schedulerName) == 0 {
		schedulerName = "default-scheduler"
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "v1",
			APIVersion: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
		},
	}

	toleration := corev1.Toleration{
		Key:      "node-role.kubernetes.io/master",
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoSchedule,
	}
	if len(pod.Spec.Tolerations) == 0 {
		pod.Spec.Tolerations = make([]corev1.Toleration, 0)
	}
	pod.Spec.Tolerations = append(pod.Spec.Tolerations, toleration)

	pod.Spec.SchedulerName = schedulerName

	if labels != nil {
		pod.SetLabels(labels)
	}

	if node != "" {
		pod.Spec.NodeName = node
	}
	return pod
}

func CreatePod(client kubernetes.Interface, pod *corev1.Pod) {
	ginkgo.By(fmt.Sprintf("Creating Pod(%s/%s)", pod.Namespace, pod.Name), func() {
		_, err := client.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf(fmt.Sprintf("Creating Pod(%s/%s)", pod.Namespace, pod.Name), err)
			gomega.Expect(apierrors.IsAlreadyExists(err)).Should(gomega.Equal(true))
		}
	})
}

func WaitPodPresentOnCluster(client kubernetes.Interface, namespace, cluster string, nodes []string, opt metav1.ListOptions) {
	ginkgo.By(fmt.Sprintf("Waiting for pod on cluster(%v)", cluster), func() {
		gomega.Eventually(func() bool {
			pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), opt)
			if err != nil {
				klog.Errorf("Failed to get pod on cluster(%s), err: %v", cluster, err)
				return false
			}

			if len(pods.Items) < 1 {
				klog.Errorf("have no pods on cluster(%s), err: %v", cluster, err)
				return false
			}

			for _, pod := range pods.Items {
				if !HasElement(pod.Spec.NodeName, nodes) {
					return false
				}
			}
			return true
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}

func RemovePodOnCluster(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Pod(%s/%s)", namespace, name), func() {
		err := client.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err == nil || apierrors.IsNotFound(err) {
			return
		}
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

func ResourceLabel(key, value string) map[string]string {
	return map[string]string{
		key: value,
	}
}
