package handlers

import (
	"fmt"
	"net"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/constants"
	"github.com/kosmos.io/kosmos/pkg/network-manager/helpers"
)

type GlobalMap struct {
	Next
}

func (h *GlobalMap) Do(c *Context) (err error) {
	gwNodes := c.Filter.GetGatewayNodes()
	epNodes := c.Filter.GetEndpointNodes()

	nodes := append(gwNodes, epNodes...)

	for _, n := range nodes {
		cluster := c.Filter.GetClusterByName(n.Spec.ClusterName)
		globalMap := cluster.Spec.GlobalCIDRsMap

		if len(globalMap) > 0 {
			for src, dst := range cluster.Spec.GlobalCIDRsMap {
				ipType := helpers.GetIPType(src)

				var vxBridge string
				if ipType == helpers.IPV6 {
					vxBridge = constants.VXLAN_BRIDGE_NAME_6
				} else if ipType == helpers.IPV4 {
					vxBridge = constants.VXLAN_BRIDGE_NAME
				}

				// todo in-cluster globalIP access
				c.Results[n.Name].Iptables = append(c.Results[n.Name].Iptables, v1alpha1.Iptables{
					Table: "nat",
					Chain: constants.IPTablesPreRoutingChain,
					Rule:  fmt.Sprintf("-d %s -i %s -j NETMAP --to %s", dst, vxBridge, src),
				})

				_, dstIP, err := net.ParseCIDR(dst)
				if err != nil {
					klog.Errorf("globalmap: invalid dstIP, err: %v", err)
					continue
				}

				c.Results[n.Name].Iptables = append(c.Results[n.Name].Iptables, v1alpha1.Iptables{
					Table: "nat",
					Chain: constants.IPTablesPostRoutingChain,
					Rule:  fmt.Sprintf("-s %s -o %s -j SNAT --to-source %s", src, vxBridge, dstIP.IP),
				})
			}
		}
	}

	return nil
}
