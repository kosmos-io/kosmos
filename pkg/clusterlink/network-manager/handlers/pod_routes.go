package handlers

import (
	"net"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network-manager/helpers"
	"github.com/kosmos.io/kosmos/pkg/constants"
	utilnet "github.com/kosmos.io/kosmos/pkg/utils/net"
)

type PodRoutes struct {
	Next
}

func (h *PodRoutes) Do(c *Context) (err error) {
	gwNodes := c.Filter.GetGatewayNodes()
	epNodes := c.Filter.GetEndpointNodes()

	nodes := append(gwNodes, epNodes...)

	for _, target := range nodes {
		cluster := c.Filter.GetClusterByName(target.Spec.ClusterName)
		var podCIDRs []string
		if cluster.IsP2P() {
			podCIDRs = target.Spec.PodCIDRs
		} else {
			podCIDRs = cluster.Status.ClusterLinkStatus.PodCIDRs
		}

		podCIDRs = FilterByIPFamily(podCIDRs, cluster.Spec.ClusterLinkOptions.IPFamily)
		podCIDRs = ConvertToGlobalCIDRs(podCIDRs, cluster.Spec.ClusterLinkOptions.GlobalCIDRsMap)
		BuildRoutes(c, target, podCIDRs)
	}

	return nil
}

func convertIPFamilyTypeToIPType(familyType v1alpha1.IPFamilyType) helpers.IPType {
	if familyType == v1alpha1.IPFamilyTypeIPV4 {
		return helpers.IPV4
	}
	return helpers.IPV6
}

func FilterByIPFamily(cidrs []string, familyType v1alpha1.IPFamilyType) (results []string) {
	if familyType == v1alpha1.IPFamilyTypeALL {
		return cidrs
	}
	specifiedIPType := convertIPFamilyTypeToIPType(familyType)
	for _, cidr := range cidrs {
		ipType := helpers.GetIPType(cidr)
		if ipType == specifiedIPType {
			results = append(results, cidr)
		}
	}
	return
}

func ConvertToGlobalCIDRs(cidrs []string, globalCIDRMap map[string]string) []string {
	if len(globalCIDRMap) == 0 {
		return cidrs
	}

	mappedCIDRs := []string{}
	for _, cidr := range cidrs {
		if dst, exists := globalCIDRMap[cidr]; exists {
			mappedCIDRs = append(mappedCIDRs, dst)
		} else {
			mappedCIDRs = append(mappedCIDRs, cidr)
		}
	}
	return mappedCIDRs
}

// ifCIDRConflictWithSelf If the target CIDR conflicts with current CIDR, do not add the route, as it will otherwise
// impact the single-cluster network.
func ifCIDRConflictWithSelf(selfCIDRs []string, tarCIDR string) bool {
	for _, cidr := range selfCIDRs {
		if utilnet.Intersect(cidr, tarCIDR) {
			return true
		}
	}
	return false
}

func SupportIPType(cluster *v1alpha1.Cluster, ipType helpers.IPType) bool {
	if cluster.Spec.ClusterLinkOptions.IPFamily == v1alpha1.IPFamilyTypeALL {
		return true
	}
	specifiedIPType := convertIPFamilyTypeToIPType(cluster.Spec.ClusterLinkOptions.IPFamily)
	return specifiedIPType == ipType
}

func BuildRoutes(ctx *Context, target *v1alpha1.ClusterNode, cidrs []string) {
	otherClusterNodes := ctx.Filter.GetAllNodesExceptCluster(target.Spec.ClusterName)

	for _, cidr := range cidrs {
		ipType := helpers.GetIPType(cidr)

		var vxBridge string
		var vxLocal string
		if ipType == helpers.IPV6 {
			vxBridge = constants.VXLAN_BRIDGE_NAME_6
			vxLocal = constants.VXLAN_LOCAL_NAME_6
		} else if ipType == helpers.IPV4 {
			vxBridge = constants.VXLAN_BRIDGE_NAME
			vxLocal = constants.VXLAN_LOCAL_NAME
		}

		targetDev := ctx.GetDeviceFromResults(target.Name, vxBridge)
		if targetDev == nil {
			klog.Warning("cannot find the target dev, nodeName: %s, devName: %s", target.Name, vxBridge)
			continue
		}

		targetIP, _, err := net.ParseCIDR(targetDev.Addr)
		if err != nil {
			klog.Warning("cannot parse target dev addr, nodeName: %s, devName: %s", target.Name, vxBridge)
			continue
		}

		for _, n := range otherClusterNodes {
			srcCluster := ctx.Filter.GetClusterByName(n.Spec.ClusterName)
			if !SupportIPType(srcCluster, ipType) {
				continue
			}

			allCIDRs := append(srcCluster.Status.ClusterLinkStatus.PodCIDRs, srcCluster.Status.ClusterLinkStatus.ServiceCIDRs...)
			if ifCIDRConflictWithSelf(allCIDRs, cidr) {
				continue
			}

			if n.IsGateway() || srcCluster.IsP2P() {
				ctx.Results[n.Name].Routes = append(ctx.Results[n.Name].Routes, v1alpha1.Route{
					CIDR: cidr,
					Gw:   targetIP.String(),
					Dev:  vxBridge,
				})
				continue
			}

			gw := ctx.Filter.GetGatewayNodeByClusterName(n.Spec.ClusterName)
			if gw == nil {
				klog.Warning("cannot find gateway node, cluster name: %s", n.Spec.ClusterName)
				continue
			}

			gwDev := ctx.GetDeviceFromResults(gw.Name, vxLocal)
			gwIP, _, err := net.ParseCIDR(gwDev.Addr)
			if err != nil {
				klog.Warning("cannot parse gw dev addr, nodeName: %s, devName: %s", gw.Name, vxLocal)
				continue
			}

			ctx.Results[n.Name].Routes = append(ctx.Results[n.Name].Routes, v1alpha1.Route{
				CIDR: cidr,
				Gw:   gwIP.String(),
				Dev:  vxLocal,
			})
		}
	}
}
