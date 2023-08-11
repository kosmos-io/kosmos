package handlers

import (
	"net"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	constants "github.com/kosmos.io/clusterlink/pkg/network"
)

type VxBridgeMacCache struct {
	Next
}

func (h *VxBridgeMacCache) Do(c *Context) (err error) {

	gwNodes := c.Filter.GetGatewayNodes()
	epNodes := c.Filter.GetEndpointNodes()

	nodes := append(gwNodes, epNodes...)

	for _, src := range nodes {
		sIPv4 := c.Filter.SupportIPv4(src)
		sIPv6 := c.Filter.SupportIPv6(src)

		var fdbs []v1alpha1.Fdb
		var arps []v1alpha1.Arp

		for _, tar := range nodes {
			if src.Name == tar.Name {
				continue
			}

			tIPv4 := c.Filter.SupportIPv4(tar)
			tIPv6 := c.Filter.SupportIPv6(tar)

			var ipTypes []v1alpha1.IPFamilyType
			if sIPv4 && tIPv4 {
				ipTypes = append(ipTypes, v1alpha1.IPFamilyTypeIPV4)
			}
			if sIPv6 && tIPv6 {
				ipTypes = append(ipTypes, v1alpha1.IPFamilyTypeIPV6)
			}

			for _, ipType := range ipTypes {
				var tarDevName string
				var tarIP string

				if ipType == v1alpha1.IPFamilyTypeIPV4 {
					tarDevName = constants.VXLAN_BRIDGE_NAME
					tarIP = tar.Spec.IP
				} else if ipType == v1alpha1.IPFamilyTypeIPV6 {
					tarDevName = constants.VXLAN_BRIDGE_NAME_6
					tarIP = tar.Spec.IP6
				}

				tarDev := c.GetDeviceFromResults(tar.Name, tarDevName)
				if tarDev == nil {
					klog.Errorf("VxBridgeMacCache, device not found nodeName: %s, devName: %s", tar.Name, tarDevName)
					continue
				}

				// 00:00:00:00:00:00 dev vx-bridge dst <remote-underlay-ip> self permanent
				fdbs = append(fdbs, v1alpha1.Fdb{
					IP:  tarIP,
					Mac: constants.ALL_ZERO_MAC,
					Dev: tarDevName,
				})

				// <remote-vxLocal-mac> dev vx-bridge dst <remote-underlay-ip> self permanent
				fdbs = append(fdbs, v1alpha1.Fdb{
					IP:  tarIP,
					Mac: tarDev.Mac,
					Dev: tarDevName,
				})

				tarDevIP, _, err := net.ParseCIDR(tarDev.Addr)
				if err != nil {
					klog.Warning("VxBridgeMacCache, cannot parse dev addr, nodeName: %s, devName: %s", tar.Name, tarDevName)
					continue
				}

				arps = append(arps, v1alpha1.Arp{
					IP:  tarDevIP.String(),
					Mac: tarDev.Mac,
					Dev: tarDevName,
				})
			}
		}

		c.Results[src.Name].Fdbs = append(c.Results[src.Name].Fdbs, fdbs...)
		c.Results[src.Name].Arps = append(c.Results[src.Name].Arps, arps...)
	}

	return
}
