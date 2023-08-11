package handlers

import (
	"net"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	constants "github.com/kosmos.io/clusterlink/pkg/network"
	"github.com/kosmos.io/clusterlink/pkg/network-manager/helpers"
)

type ServiceRoutes struct {
	Next
}

func (h *ServiceRoutes) Do(c *Context) (err error) {

	gwNodes := c.Filter.GetGatewayNodes()

	for _, target := range gwNodes {
		cluster := c.Filter.GetClusterByName(target.Spec.ClusterName)
		otherClusterNodes := c.Filter.GetAllNodesExceptCluster(target.Spec.ClusterName)
		serviceCIDRs := cluster.Status.ServiceCIDRs

		for _, cidr := range serviceCIDRs {
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

			targetDev := c.GetDeviceFromResults(target.Name, vxBridge)
			targetIP, _, err := net.ParseCIDR(targetDev.Addr)
			if err != nil {
				klog.Warning("ServiceRoutesHandler, cannot parse target dev addr, nodeName: %s, devName: %s", target.Name, vxBridge)
				continue
			}

			for _, n := range otherClusterNodes {
				srcCluster := c.Filter.GetClusterByName(n.Spec.ClusterName)
				if n.IsGateway() || srcCluster.IsP2P() {
					c.Results[n.Name].Routes = append(c.Results[n.Name].Routes, v1alpha1.Route{
						CIDR: cidr,
						Gw:   targetIP.String(),
						Dev:  vxBridge,
					})
					continue
				}

				gw := c.Filter.GetGatewayNodeByClusterName(n.Spec.ClusterName)
				gwDev := c.GetDeviceFromResults(gw.Name, vxLocal)
				gwIP, _, err := net.ParseCIDR(gwDev.Addr)
				if err != nil {
					klog.Warning("ServiceRoutesHandler, cannot parse gw dev addr, nodeName: %s, devName: %s", gw.Name, vxLocal)
					continue
				}

				c.Results[n.Name].Routes = append(c.Results[n.Name].Routes, v1alpha1.Route{
					CIDR: cidr,
					Gw:   gwIP.String(),
					Dev:  vxLocal,
				})
			}
		}
	}

	return
}
