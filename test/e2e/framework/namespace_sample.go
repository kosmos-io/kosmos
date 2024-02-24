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

func GetNamespace(ns string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "v1",
			APIVersion: "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   ns,
			Labels: labels,
		},
	}
}

func CreateNamespace(client kubernetes.Interface, ns *corev1.Namespace) {
	ginkgo.By(fmt.Sprintf("Create namespace(%s)", ns), func() {
		_, err := client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("create namespace occur error ：", err)
			gomega.Expect(apierrors.IsAlreadyExists(err)).Should(gomega.Equal(true))
		}
	})
}
