// nolint:dupl
package e2e

import (
	"fmt"
	"reflect"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/test/e2e/framework"
)

const (
	ONE2CLUSTER = "-one2cluster"
	ONE2NODE    = "-one2node"
	ONE2PARTY   = "-one2party"
)

var (
	one2Cluster     *kosmosv1alpha1.Cluster
	one2Node        *kosmosv1alpha1.Cluster
	one2Party       *kosmosv1alpha1.Cluster
	partyNodeNames  []string
	memberNodeNames []string
)

var _ = ginkgo.Describe("Test leaf node mode -- one2cluster, one2node, one2party", func() {
	ginkgo.BeforeEach(func() {
		clusters, err := framework.FetchClusters(hostClusterLinkClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(clusters).ShouldNot(gomega.BeEmpty())
		partyNodeNames = make([]string, 0)
		memberNodeNames = make([]string, 0)

		for _, cluster := range clusters {
			if cluster.Name == "cluster-member3" {
				nodes, err := framework.FetchNodes(thirdKubeClient)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				cluster.ResourceVersion = ""

				one2Cluster = cluster.DeepCopy()
				one2Cluster.Name += ONE2CLUSTER
				one2Cluster.Spec.ClusterTreeOptions.Enable = true
				one2Cluster.Spec.ClusterTreeOptions.LeafModels = nil

				one2Node = cluster.DeepCopy()
				one2Node.Name += ONE2NODE
				one2Node.Spec.ClusterTreeOptions.Enable = true

				one2Party = cluster.DeepCopy()
				one2Party.Name += ONE2PARTY

				nodeLeafModels := make([]kosmosv1alpha1.LeafModel, 0)
				for i, node := range nodes {
					if i < 2 {
						nodeLabels := node.Labels
						if nodeLabels == nil {
							nodeLabels = make(map[string]string)
						}
						// nolint:gosec
						nodeLabels["test-leaf-party-mode"] = "yes"
						node.SetLabels(nodeLabels)
						node.ResourceVersion = ""
						err = framework.UpdateNodeLabels(thirdKubeClient, node)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					}

					nodeLeaf := kosmosv1alpha1.LeafModel{
						LeafNodeName: one2Node.Name,
						Taints: []corev1.Taint{
							{
								Effect: utils.KosmosNodeTaintEffect,
								Key:    utils.KosmosNodeTaintKey,
								Value:  utils.KosmosNodeValue,
							},
							{
								Effect: utils.KosmosNodeTaintEffect,
								Key:    "test-node/e2e",
								Value:  "leafnode",
							},
						},
						NodeSelector: kosmosv1alpha1.NodeSelector{
							NodeName:      node.Name,
							LabelSelector: nil,
						},
					}
					nodeLeafModels = append(nodeLeafModels, nodeLeaf)
					memberNodeNames = append(memberNodeNames, node.Name)

				}
				one2Node.Spec.ClusterTreeOptions.LeafModels = nodeLeafModels

				partyLeaf := kosmosv1alpha1.LeafModel{
					LeafNodeName: one2Party.Name,
					Taints: []corev1.Taint{
						{
							Effect: utils.KosmosNodeTaintEffect,
							Key:    utils.KosmosNodeTaintKey,
							Value:  utils.KosmosNodeValue,
						},
						{
							Effect: utils.KosmosNodeTaintEffect,
							Key:    "test-node/e2e",
							Value:  "leafnode",
						},
					},
					NodeSelector: kosmosv1alpha1.NodeSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-leaf-party-mode": "yes",
							},
						},
					},
				}

				partyNodeNames = append(partyNodeNames, fmt.Sprintf("%v%v%v", utils.KosmosNodePrefix, partyLeaf.LeafNodeName, "-0"))
				one2Party.Spec.ClusterTreeOptions.LeafModels = []kosmosv1alpha1.LeafModel{partyLeaf}

				break
			}
		}
	})

	ginkgo.Context("Test one2cluster mode", func() {
		var deploy *appsv1.Deployment
		ginkgo.BeforeEach(func() {
			err := framework.DeleteClusters(hostClusterLinkClient, one2Cluster.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
			err = framework.CreateClusters(hostClusterLinkClient, one2Cluster)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			framework.WaitNodePresentOnCluster(hostKubeClient, utils.KosmosNodePrefix+one2Cluster.GetName())

		})

		ginkgo.It("Test one2cluster mode", func() {
			ginkgo.By("Test one2cluster mode", func() {
				nodeNameInRoot := utils.KosmosNodePrefix + one2Cluster.GetName()
				nodes := []string{nodeNameInRoot}
				deployName := one2Cluster.GetName() + "-nginx"
				rp := int32(1)
				deploy = framework.NewDeployment(corev1.NamespaceDefault, deployName, framework.SchedulerName, framework.ResourceLabel("app", deployName), &rp, nodes, true)
				framework.RemoveDeploymentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name)
				framework.CreateDeployment(hostKubeClient, deploy)

				framework.WaitDeploymentPresentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name, one2Cluster.Name)

				opt := metav1.ListOptions{
					LabelSelector: fmt.Sprintf("app=%v", deployName),
				}
				framework.WaitPodPresentOnCluster(hostKubeClient, deploy.Namespace, one2Cluster.Name, nodes, opt)
				framework.WaitPodPresentOnCluster(thirdKubeClient, deploy.Namespace, one2Cluster.Name, memberNodeNames, opt)
			})
		})
		ginkgo.AfterEach(func() {
			if deploy != nil && !reflect.DeepEqual(deploy, appsv1.Deployment{}) {
				framework.RemoveDeploymentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name)
			}

			err := framework.DeleteClusters(hostClusterLinkClient, one2Cluster.Name)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			err = framework.DeleteNode(hostKubeClient, utils.KosmosNodePrefix+one2Cluster.GetName())
			if err != nil && !apierrors.IsNotFound(err) {
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
		})
	})

	ginkgo.Context("Test one2node mode", func() {
		var deploy *appsv1.Deployment
		ginkgo.BeforeEach(func() {
			err := framework.DeleteClusters(hostClusterLinkClient, one2Node.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
			err = framework.CreateClusters(hostClusterLinkClient, one2Node)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			// nolint:gosec
			if len(memberNodeNames) > 0 {
				framework.WaitNodePresentOnCluster(hostKubeClient, memberNodeNames[0])
			}
		})

		ginkgo.It("Test one2node mode", func() {
			ginkgo.By("Test one2cluster mode", func() {
				deployName := one2Node.GetName() + "-nginx"
				rp := int32(1)
				deploy = framework.NewDeployment(corev1.NamespaceDefault, deployName, framework.SchedulerName, framework.ResourceLabel("app", deployName), &rp, memberNodeNames, true)
				framework.RemoveDeploymentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name)
				framework.CreateDeployment(hostKubeClient, deploy)

				framework.WaitDeploymentPresentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name, one2Node.Name)

				opt := metav1.ListOptions{
					LabelSelector: fmt.Sprintf("app=%v", deployName),
				}
				framework.WaitPodPresentOnCluster(hostKubeClient, deploy.Namespace, one2Node.Name, memberNodeNames, opt)
				framework.WaitPodPresentOnCluster(thirdKubeClient, deploy.Namespace, one2Node.Name, memberNodeNames, opt)
			})
		})

		ginkgo.AfterEach(func() {
			if deploy != nil && !reflect.DeepEqual(deploy, appsv1.Deployment{}) {
				framework.RemoveDeploymentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name)
			}

			err := framework.DeleteClusters(hostClusterLinkClient, one2Node.Name)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			if len(memberNodeNames) > 0 {
				for _, node := range memberNodeNames {
					err = framework.DeleteNode(hostKubeClient, node)
					if err != nil && !apierrors.IsNotFound(err) {
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					}
				}
			}
		})
	})

	ginkgo.Context("Test one2party mode", func() {
		var deploy *appsv1.Deployment
		ginkgo.BeforeEach(func() {
			err := framework.DeleteClusters(hostClusterLinkClient, one2Party.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
			err = framework.CreateClusters(hostClusterLinkClient, one2Party)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			// nolint:gosec
			if len(partyNodeNames) > 0 {
				framework.WaitNodePresentOnCluster(hostKubeClient, partyNodeNames[0])
			}
		})

		ginkgo.It("Test one2party mode", func() {
			ginkgo.By("Test one2party mode", func() {
				deployName := one2Party.GetName() + "-nginx"
				rp := int32(1)
				deploy = framework.NewDeployment(corev1.NamespaceDefault, deployName, framework.SchedulerName, framework.ResourceLabel("app", deployName), &rp, partyNodeNames, true)
				framework.RemoveDeploymentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name)
				framework.CreateDeployment(hostKubeClient, deploy)

				framework.WaitDeploymentPresentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name, one2Party.Name)

				opt := metav1.ListOptions{
					LabelSelector: fmt.Sprintf("app=%v", deployName),
				}
				framework.WaitPodPresentOnCluster(hostKubeClient, deploy.Namespace, one2Party.Name, partyNodeNames, opt)
				framework.WaitPodPresentOnCluster(thirdKubeClient, deploy.Namespace, one2Party.Name, memberNodeNames, opt)
			})
		})
		ginkgo.AfterEach(func() {
			if deploy != nil && !reflect.DeepEqual(deploy, appsv1.Deployment{}) {
				framework.RemoveDeploymentOnCluster(hostKubeClient, deploy.Namespace, deploy.Name)
			}

			err := framework.DeleteClusters(hostClusterLinkClient, one2Party.Name)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			if len(partyNodeNames) > 0 {
				for _, node := range partyNodeNames {
					err = framework.DeleteNode(hostKubeClient, node)
					if err != nil && !apierrors.IsNotFound(err) {
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					}
				}
			}
		})
	})
})
