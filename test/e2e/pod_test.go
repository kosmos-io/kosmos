package e2e

import (
	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/kosmos.io/kosmos/test/e2e/framework"
	"github.com/kosmos.io/kosmos/test/helper"
)

var _ = ginkgo.Describe("pod testing in kosmos", func() {

	var knodeName string
	var namespaceName string
	var namespace *v1.Namespace
	var podName string
	var pod *v1.Pod
	knodeName = "knode-test-kc"
	namespaceName = "kosmos-e2e-ns"

	ginkgo.BeforeEach(func() {
		namespace = helper.NewNamespace(namespaceName)
		framework.CreateNamespace(kubeClient, namespace)
		framework.WaitNamespacePresentOnKosmos(kubeClient, namespaceName, func(nameSpace *v1.Namespace) bool {
			return true
		})
	})

	ginkgo.AfterEach(func() {
		framework.RemoveNamespace(kubeClient, namespaceName)
		framework.WaitNameSpaceDisappearOnKosmos(kubeClient, namespaceName)
	})

	ginkgo.When("creating a pod in kosmos-apiserver", func() {
		ginkgo.BeforeEach(func() {
			podName = "pod-e2e-ns-" + rand.String(3)
			pod = helper.NewPod(knodeName, namespaceName, podName)
			framework.CreatePod(kubeClient, pod)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePod(kubeClient, namespaceName, podName)
		})

		ginkgo.It("pod should be presented on knodes", func() {
			framework.WaitPodPresentOnKnodes(kosmosClient, knodeName, namespaceName, podName, func(pod *v1.Pod) bool {
				return true
			})
		})
	})

	ginkgo.When("deleting a pod in kosmos-apiserver", func() {
		ginkgo.BeforeEach(func() {
			podName = "pod-e2e-ns-" + rand.String(3)
			pod = helper.NewPod(knodeName, namespaceName, podName)
			framework.CreatePod(kubeClient, pod)
		})

		ginkgo.It("pod should be disappeared on knodes", func() {
			framework.WaitPodDisappearOnKnodes(kosmosClient, knodeName, namespaceName, podName)
		})
	})
})
