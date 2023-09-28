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

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

// CreatePod create Pod.
func CreatePod(client kubernetes.Interface, pod *corev1.Pod) {
	ginkgo.By(fmt.Sprintf("Creating Pod(%s/%s)", pod.Namespace, pod.Name), func() {
		_, err := client.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemovePod delete Pod.
func RemovePod(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Pod(%s/%s)", namespace, name), func() {
		err := client.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitPodPresentOnKnodes wait pod present on knodes sync with fit func.
func WaitPodPresentOnKnodes(kosmosClient versioned.Interface, knode, namespace, name string, fit func(pod *corev1.Pod) bool) {
	knodeClient := GetKnodeClient(kosmosClient, knode)
	gomega.Expect(knodeClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for pod(%s/%s) synced on knode(%s)", namespace, name, knode)
	gomega.Eventually(func() bool {
		pod, err := knodeClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(pod)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitPodDisappearOnKnodes wait pod disappear on knodes until timeout.
func WaitPodDisappearOnKnodes(kosmosClient versioned.Interface, knode, namespace, name string) {
	knodeClient := GetKnodeClient(kosmosClient, knode)
	gomega.Expect(knodeClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for pod(%s/%s) disappear on knode(%s)", namespace, name, knode)
	gomega.Eventually(func() bool {
		_, err := knodeClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get pod(%s/%s) on knode(%s), err: %v", namespace, name, knode, err)
		return false
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}
