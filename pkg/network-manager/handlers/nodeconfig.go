package handlers

import (
	"sort"

	"encoding/json"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/network-manager/helpers"
)

// NodeConfig network configuration of the node
type NodeConfig struct {
	Devices  []v1alpha1.Device   `json:"devices,omitempty"`
	Routes   []v1alpha1.Route    `json:"routes,omitempty"`
	Iptables []v1alpha1.Iptables `json:"iptables,omitempty"`
	Fdbs     []v1alpha1.Fdb      `json:"fdbs,omitempty"`
	Arps     []v1alpha1.Arp      `json:"arps,omitempty"`
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
		Devices:  c.Devices,
		Routes:   c.Routes,
		Iptables: c.Iptables,
		Fdbs:     c.Fdbs,
		Arps:     c.Arps,
	}
}

func (c *NodeConfig) Sort() {
	sort.Sort(helpers.DevicesSorter(c.Devices))
	sort.Sort(helpers.FdbSorter(c.Fdbs))
	sort.Sort(helpers.IptablesSorter(c.Iptables))
	sort.Sort(helpers.ArpSorter(c.Arps))
	sort.Sort(helpers.RouteSorter(c.Routes))
}
