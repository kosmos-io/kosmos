// nolint:dupl
package e2e

import (
	"fmt"
	"reflect"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kosmosutils "github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/test/e2e/framework"
)

var (
	mixNodeNames  []string
	hostNodeNames []string
	leafNodeNames []string
)

var _ = ginkgo.Describe("Kosmos scheduler plugin -- distribution policy", func() {
	ginkgo.BeforeEach(func() {
		nodes, err := framework.FetchNodes(hostKubeClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(nodes).ShouldNot(gomega.BeEmpty())
		mixNodeNames = make([]string, 0)
		hostNodeNames = make([]string, 0)
		leafNodeNames = make([]string, 0)

		for _, node := range nodes {
			labels := node.GetLabels()
			if labels[kosmosutils.NodeRoleLabel] == kosmosutils.NodeRoleValue {
				leafNodeNames = append(leafNodeNames, node.Name)
			} else {
				hostNodeNames = append(hostNodeNames, node.Name)
			}
			mixNodeNames = append(mixNodeNames, node.Name)
		}

		nameScopeNs := framework.GetNamespace(framework.NameScopeNs, nil)
		framework.CreateNamespace(hostKubeClient, nameScopeNs)
		namespaceScopeNs := framework.GetNamespace(framework.NamespaceScopeNs, nil)
		framework.CreateNamespace(hostKubeClient, namespaceScopeNs)

		dp := framework.NewDistributionPolicy()
		framework.CreateDistributionPolicy(hostClusterLinkClient, framework.NameScopeNs, dp)
		cdp := framework.NewClusterDistributionPolicy()
		framework.CreateClusterDistributionPolicy(hostClusterLinkClient, cdp)
	})

	ginkgo.AfterEach(func() {

	})

	ginkgo.Context("Test distribution policy", func() {
		var podLabelDeploy *appsv1.Deployment
		var namePod *corev1.Pod
		var prefixPod *corev1.Pod

		var deploy *appsv1.Deployment

		ginkgo.BeforeEach(func() {

		})
		ginkgo.AfterEach(func() {
			if podLabelDeploy != nil && !reflect.DeepEqual(podLabelDeploy, appsv1.Deployment{}) {
				framework.RemoveDeploymentOnCluster(hostKubeClient, podLabelDeploy.Namespace, podLabelDeploy.Name)
			}
			if namePod != nil && !reflect.DeepEqual(namePod, corev1.Pod{}) {
				framework.RemovePodOnCluster(hostKubeClient, namePod.Namespace, namePod.Name)
			}
			if prefixPod != nil && !reflect.DeepEqual(prefixPod, corev1.Pod{}) {
				framework.RemovePodOnCluster(hostKubeClient, prefixPod.Namespace, prefixPod.Name)
			}
			if deploy != nil && !reflect.DeepEqual(deploy, appsv1.Deployment{}) {
				framework.RemoveDeploymentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name)
			}
		})

		ginkgo.It("Test distribution policy --  namespace-level -- pod -- name", func() {
			ginkgo.By(fmt.Sprintf("Test Pod(%v/%v) in leaf node ", framework.NameScopeNs, framework.PodName), func() {
				namePod = framework.NewPod(framework.NameScopeNs, framework.PodName, framework.SchedulerName, nil, "")
				framework.CreatePod(hostKubeClient, namePod)
				opt := metav1.ListOptions{
					FieldSelector: fmt.Sprintf("metadata.name=%s", namePod.Name),
				}
				framework.WaitPodPresentOnCluster(hostKubeClient, namePod.Namespace, "leaf", leafNodeNames, opt)
			})
		})

		ginkgo.It("Test distribution policy --  namespace-level -- pod -- namePrefix", func() {
			ginkgo.By(fmt.Sprintf("Test PodNamePrefix(%v/%v) in host node ", framework.NameScopeNs, framework.PodNamePrefix), func() {
				prefixPod = framework.NewPod(framework.NameScopeNs, framework.PodNamePrefix+"-01", framework.SchedulerName, nil, "")
				framework.CreatePod(hostKubeClient, prefixPod)
				opt := metav1.ListOptions{
					FieldSelector: fmt.Sprintf("metadata.name=%s", prefixPod.Name),
				}
				framework.WaitPodPresentOnCluster(hostKubeClient, prefixPod.Namespace, "host", hostNodeNames, opt)
			})
		})

		ginkgo.It("Test distribution policy --  namespace-level -- pod -- label", func() {
			ginkgo.By(fmt.Sprintf("Test Deployment(%v/%v) by label(%v) in mix node ", framework.NameScopeNs, framework.PodNameLabelScopeNs, framework.PodLabel), func() {
				rp := int32(3)
				podLabelDeploy = framework.NewDeployment(framework.NameScopeNs, framework.PodNameLabelScopeNs, framework.SchedulerName, framework.PodLabel, &rp, nil, false)
				framework.CreateDeployment(hostKubeClient, podLabelDeploy)
				opt := metav1.ListOptions{
					LabelSelector: "example-distribution-policy=nginx",
				}
				framework.WaitDeploymentPresentOnCluster(hostKubeClient, podLabelDeploy.Namespace, podLabelDeploy.Name, "mix")
				framework.WaitPodPresentOnCluster(hostKubeClient, podLabelDeploy.Namespace, "mix", mixNodeNames, opt)
			})
		})

		ginkgo.It("Test cluster distribution policy --  cluster-level -- pod -- name", func() {
			ginkgo.By(fmt.Sprintf("Test Pod(%v/%v) in mix node ", framework.NamespaceScopeNs, framework.PodNameClusterLevel), func() {
				namePod = framework.NewPod(framework.NamespaceScopeNs, framework.PodNameClusterLevel, framework.SchedulerName, nil, "")
				framework.CreatePod(hostKubeClient, namePod)
				opt := metav1.ListOptions{
					FieldSelector: fmt.Sprintf("metadata.name=%s", namePod.Name),
				}
				framework.WaitPodPresentOnCluster(hostKubeClient, namePod.Namespace, "mix", mixNodeNames, opt)
			})
		})
	})
})
