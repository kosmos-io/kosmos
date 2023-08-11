package handlers

import (
	"net"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	constants "github.com/kosmos.io/clusterlink/pkg/network"
)

type VxLocalMacCache struct {
	Next
}

func (h *VxLocalMacCache) Do(c *Context) (err error) {

	nodes := c.Filter.GetInternalNodes()

	for _, node := range nodes {
		var fdbs []v1alpha1.Fdb
		var arps []v1alpha1.Arp

		var ipTypes []v1alpha1.IPFamilyType
		supportIPv4 := c.Filter.SupportIPv4(node)
		supportIPv6 := c.Filter.SupportIPv6(node)
		if supportIPv4 {
			ipTypes = append(ipTypes, v1alpha1.IPFamilyTypeIPV4)
		}
		if supportIPv6 {
			ipTypes = append(ipTypes, v1alpha1.IPFamilyTypeIPV6)
		}

		gw := c.Filter.GetGatewayNodeByClusterName(node.Spec.ClusterName)
		for _, ipType := range ipTypes {
			var gwDevName string
			var gwIP string

			if ipType == v1alpha1.IPFamilyTypeIPV4 {
				gwDevName = constants.VXLAN_LOCAL_NAME
				gwIP = gw.Spec.IP
			} else if ipType == v1alpha1.IPFamilyTypeIPV6 {
				gwDevName = constants.VXLAN_LOCAL_NAME_6
				gwIP = gw.Spec.IP6
			}

			gwDev := c.GetDeviceFromResults(gw.Name, gwDevName)
			if gwDev == nil {
				klog.Errorf("VxLocalMacCache, device not found devName: %s, nodeName: %s", gwDevName, gw.Name)
				continue
			}

			// 00:00:00:00:00:00 dev vx-local dst <remote-underlay-ip> self permanent
			// <remote-vxLocal-mac> dev vx-local dst <remote-underlay-ip> self permanent
			fdbs = append(fdbs, []v1alpha1.Fdb{
				{
					IP:  gwIP,
					Mac: constants.ALL_ZERO_MAC,
					Dev: gwDevName,
				},
				{
					IP:  gwIP,
					Mac: gwDev.Mac,
					Dev: gwDevName,
				},
			}...)

			devIP, _, err := net.ParseCIDR(gwDev.Addr)
			if err != nil {
				klog.Warning("VxLocalMacCache, cannot parse dev addr, nodeName: %s, devName: %s", gw.Name, gwDev.Name)
				continue
			}
			// arp -s <remote-vxLocal-ip> <remote-vxLocal-mac> -i <local-dev>
			arps = append(arps, v1alpha1.Arp{
				IP:  devIP.String(),
				Mac: gwDev.Mac,
				Dev: gwDevName,
			})

		}

		c.Results[node.Name].Fdbs = append(c.Results[node.Name].Fdbs, fdbs...)
		c.Results[node.Name].Arps = append(c.Results[node.Name].Arps, arps...)
	}

	return
}
