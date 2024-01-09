package handlers

import (
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network"
)

type CNISupport struct {
	Next
}

func (h *CNISupport) Do(c *Context) (err error) {
	nonMasqClusters, otherClusters := c.Filter.GetClusterByCNI(clusterlink.NonMasqCNISlice)
	allClustes := append(nonMasqClusters, otherClusters...)
	for _, nonMasqCluster := range nonMasqClusters {
		var targetIPset []v1alpha1.IPset
		for _, otherClusters := range allClustes {
			if otherClusters.Name != nonMasqCluster.Name {
				for _, cidr := range otherClusters.Status.ClusterLinkStatus.PodCIDRs {
					targetIPset = append(targetIPset, v1alpha1.IPset{
						Name: network.KosmosIPsetVoidMasq,
						CIDR: cidr,
					})
				}
			}
		}
		klog.Infof("flannel cluster name: %s, ipset: %v", nonMasqCluster.Name, targetIPset)
		targetNodes := c.Filter.GetAllNodesByClusterName(nonMasqCluster.Name)
		for _, node := range targetNodes {
			c.Results[node.Name].IPsetsAvoidMasq = targetIPset
		}
	}

	return nil
}
