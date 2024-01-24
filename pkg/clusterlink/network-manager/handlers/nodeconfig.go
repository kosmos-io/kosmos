package handlers

import (
	"encoding/json"
	"sort"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network-manager/helpers"
)

// NodeConfig network configuration of the node
type NodeConfig struct {
	Devices         []v1alpha1.Device     `json:"devices,omitempty"`
	Routes          []v1alpha1.Route      `json:"routes,omitempty"`
	Iptables        []v1alpha1.Iptables   `json:"iptables,omitempty"`
	Fdbs            []v1alpha1.Fdb        `json:"fdbs,omitempty"`
	Arps            []v1alpha1.Arp        `json:"arps,omitempty"`
	XfrmPolicies    []v1alpha1.XfrmPolicy `json:"xfrmpolicies,omitempty"`
	XfrmStates      []v1alpha1.XfrmState  `json:"xfrmstates,omitempty"`
	IPsetsAvoidMasq []v1alpha1.IPset      `json:"ipsetsavoidmasq,omitempty"`
}

func (c *NodeConfig) ToString() string {
	b, err := json.Marshal(c)
	if err != nil {
		klog.Errorf("cannot convert nodeConfig to json string")
	}
	return string(b)
}

func (c *NodeConfig) ToJson() ([]byte, error) {
	return json.Marshal(c)
}

func (c *NodeConfig) ConvertToNodeConfigSpec() v1alpha1.NodeConfigSpec {
	return v1alpha1.NodeConfigSpec{
		Devices:          c.Devices,
		Routes:           c.Routes,
		Iptables:         c.Iptables,
		Fdbs:             c.Fdbs,
		Arps:             c.Arps,
		XfrmStates:       c.XfrmStates,
		XfrmPolicies:     c.XfrmPolicies,
		IPsetsAvoidMasqs: c.IPsetsAvoidMasq,
	}
}

func (c *NodeConfig) Sort() {
	sort.Sort(helpers.DevicesSorter(c.Devices))
	sort.Sort(helpers.FdbSorter(c.Fdbs))
	sort.Sort(helpers.IptablesSorter(c.Iptables))
	sort.Sort(helpers.ArpSorter(c.Arps))
	sort.Sort(helpers.RouteSorter(c.Routes))
	sort.Sort(helpers.XfrmPolicySorter(c.XfrmPolicies))
	sort.Sort(helpers.XfrmStateSorter(c.XfrmStates))
	sort.Sort(helpers.IPSetSorter(c.IPsetsAvoidMasq))
}
