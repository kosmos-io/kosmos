package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/utils/role"
)

var _ = ginkgo.Describe("elector testing", func() {
	ginkgo.Context("gateway role add test", func() {
		ginkgo.It("Check if gateway role gateway role is set correctly", func() {
			gomega.Eventually(func(g gomega.Gomega) (bool, error) {
				clusterNodes, err := clusterLinkClient.ClusterlinkV1alpha1().ClusterNodes().List(context.TODO(), metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				return hasGatewayRoleOnly(clusterNodes.Items), nil
			}, 60, 10).Should(gomega.Equal(true))
		})
	})
})

func hasGatewayRoleOnly(nodes []clusterlinkv1alpha1.ClusterNode) bool {
	clusterToNodeMap := make(map[string][]clusterlinkv1alpha1.ClusterNode)

	// Group the nodes by cluster name
	for _, node := range nodes {
		clusterName := node.Spec.ClusterName
		if len(clusterName) > 0 {
			if _, ok := clusterToNodeMap[clusterName]; !ok {
				clusterToNodeMap[clusterName] = []clusterlinkv1alpha1.ClusterNode{node}
			} else {
				clusterToNodeMap[clusterName] = append(clusterToNodeMap[clusterName], node)
			}
		}
	}

	// Check if each cluster has only one node with GatewayRole
	for _, nodes := range clusterToNodeMap {
		gatewayCount := 0
		for _, node := range nodes {
			if role.HasRole(node, clusterlinkv1alpha1.RoleGateway) {
				gatewayCount++
			}
		}
		if gatewayCount != 1 {
			return false
		}
	}

	return true
}
