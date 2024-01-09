package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	// PollInterval defines the interval time for a poll operation.
	PollInterval = 15 * time.Second

	// PollTimeout defines the time after which the poll operation times out.
	PollTimeout = 180 * time.Second
)

func NewDeployment(namespace, name string, replicas *int32, nodes []string) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},

		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},

		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,

			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},

				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "kosmos.io/node",
							Operator: corev1.TolerationOpEqual,
							Value:    "true",
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "test-node/e2e",
							Operator: corev1.TolerationOpEqual,
							Value:    "leafnode",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},

					HostNetwork: true,

					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/hostname",
												Operator: corev1.NodeSelectorOpIn,
												Values:   nodes,
											},
										},
									},
								},
							},
						},
					},

					Containers: []corev1.Container{
						{
							Name:  "nginx-container",
							Image: "registry.paas/cmss/nginx:1.14.2",

							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      "TCP",
								},
							},

							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				},
			},
		},
	}
}

func CreateDeployment(client kubernetes.Interface, deployment *appsv1.Deployment) {
	ginkgo.By(fmt.Sprintf("Creating Deployment(%s/%s)", deployment.Namespace, deployment.Name), func() {
		_, err := client.AppsV1().Deployments(deployment.Namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("create deployment occur error ：", err)
			gomega.Expect(apierrors.IsAlreadyExists(err)).Should(gomega.Equal(true))
		}
	})
}

func WaitDeploymentPresentOnCluster(client kubernetes.Interface, namespace, name, cluster string) {
	ginkgo.By(fmt.Sprintf("Waiting for deployment(%v/%v) on cluster(%v)", namespace, name, cluster), func() {
		gomega.Eventually(func() bool {
			_, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to get deployment(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
				return false
			}
			return true
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}

func RemoveDeploymentOnCluster(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Deployment(%s/%s)", namespace, name), func() {
		err := client.AppsV1().Deployments(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err == nil || apierrors.IsNotFound(err) {
			return
		}
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

func HasElement(str string, strs []string) bool {
	for _, e := range strs {
		if e == str {
			return true
		}
	}
	return false
}

func WaitPodPresentOnCluster(client kubernetes.Interface, namespace, cluster string, nodes []string, opt metav1.ListOptions) {
	ginkgo.By(fmt.Sprintf("Waiting for pod of the deployment on cluster(%v)", cluster), func() {
		gomega.Eventually(func() bool {
			pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), opt)
			if err != nil {
				klog.Errorf("Failed to get pod on cluster(%s), err: %v", cluster, err)
				return false
			}

			for _, pod := range pods.Items {
				if HasElement(pod.Spec.NodeName, nodes) {
					return true
				}
			}
			return false
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}
