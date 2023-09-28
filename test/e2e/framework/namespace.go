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

// CreateNamespace create Namespace.
func CreateNamespace(client kubernetes.Interface, namespace *corev1.Namespace) {
	ginkgo.By(fmt.Sprintf("Creating Namespace(%s)", namespace.Name), func() {
		_, err := client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveNamespace delete Namespace.
func RemoveNamespace(client kubernetes.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Removing Namespace(%s)", name), func() {
		err := client.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitNamespacePresentOnKosmos wait namespace present on kosmos-apiserver until timeout.
func WaitNamespacePresentOnKosmos(client kubernetes.Interface, name string, fit func(nameSpace *corev1.Namespace) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for nameSpace(%s) on kosmos-apiserver", name), func() {
		gomega.Eventually(func() bool {
			nameSpace, err := client.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return fit(nameSpace)
		}, pollTimeout, pollInterval).Should(gomega.Equal(true))
	})
}

// WaitNameSpaceDisappearOnKosmos wait namespace disappear on kosmos-apiserver until timeout.
func WaitNameSpaceDisappearOnKosmos(client kubernetes.Interface, name string) {
	gomega.Eventually(func() bool {
		_, err := client.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get nameSpace(%s) on kosmos-apiserver, err: %v", name, err)
		return false
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}
