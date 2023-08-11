package handlers

import (
	"fmt"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	constants "github.com/kosmos.io/clusterlink/pkg/network"
)

type HostNetwork struct {
	Next
}

func (h *HostNetwork) Do(c *Context) (err error) {

	gwNodes := c.Filter.GetGatewayNodes()

	for _, n := range gwNodes {
		cluster := c.Filter.GetClusterByName(n.Spec.ClusterName)
		if cluster.IsP2P() {
			continue
		}

		if c.Filter.SupportIPv4(n) {
			c.Results[n.Name].Iptables = append(c.Results[n.Name].Iptables, v1alpha1.Iptables{
				Table: "nat",
				Chain: constants.IPTablesPostRoutingChain,
				Rule:  fmt.Sprintf("-s %s -o %s -j MASQUERADE", cluster.Spec.LocalCIDRs.IP, constants.VXLAN_BRIDGE_NAME),
			})

			c.Results[n.Name].Iptables = append(c.Results[n.Name].Iptables, v1alpha1.Iptables{
				Table: "nat",
				Chain: constants.IPTablesPostRoutingChain,
				Rule:  fmt.Sprintf("-s %s -j MASQUERADE", cluster.Spec.BridgeCIDRs.IP),
			})
		}

		if c.Filter.SupportIPv6(n) {
			c.Results[n.Name].Iptables = append(c.Results[n.Name].Iptables, v1alpha1.Iptables{
				Table: "nat",
				Chain: constants.IPTablesPostRoutingChain,
				Rule:  fmt.Sprintf("-s %s -o %s -j MASQUERADE", cluster.Spec.LocalCIDRs.IP6, constants.VXLAN_BRIDGE_NAME_6),
			})

			c.Results[n.Name].Iptables = append(c.Results[n.Name].Iptables, v1alpha1.Iptables{
				Table: "nat",
				Chain: constants.IPTablesPostRoutingChain,
				Rule:  fmt.Sprintf("-s %s -j MASQUERADE", cluster.Spec.BridgeCIDRs.IP6),
			})
		}
	}

	return nil
}
