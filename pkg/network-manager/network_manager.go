package network_manager

import (
	"encoding/json"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/network-manager/handlers"
	"github.com/kosmos.io/clusterlink/pkg/network-manager/helpers"
)

type Manager struct {
	NodeConfigs map[string]*handlers.NodeConfig
}

func NewManager() *Manager {
	return &Manager{}
}

// CalculateNetworkConfigs Calculate the network configuration required for each node
func (n *Manager) CalculateNetworkConfigs(clusters []v1alpha1.Cluster, clusterNodes []v1alpha1.ClusterNode, nodeConfigs []v1alpha1.NodeConfig) (map[string]*handlers.NodeConfig, error) {

	filter := helpers.NewFilter(clusterNodes, clusters, nodeConfigs)

	c := &handlers.Context{
		Filter: filter,
	}

	rootHandler := &handlers.RootHandler{}
	rootHandler.
		SetNext(&handlers.InitNodes{}).
		SetNext(&handlers.VxLocalNetwork{}).
		SetNext(&handlers.VxBridgeNetwork{}).
		SetNext(&handlers.ServiceRoutes{}).
		SetNext(&handlers.PodRoutes{}).
		SetNext(&handlers.VxLocalMacCache{}).
		SetNext(&handlers.VxBridgeMacCache{}).
		SetNext(&handlers.GlobalMap{}).
		SetNext(&handlers.HostNetwork{})

	if err := rootHandler.Run(c); err != nil {
		return nil, fmt.Errorf("filed to calculate network config, err: %v", err)
	}

	n.NodeConfigs = c.Results
	n.SortConfigs()

	return c.Results, nil
}

func (n *Manager) GetConfigs() map[string]*handlers.NodeConfig {
	return n.NodeConfigs
}

func (n *Manager) GetConfigsByNodeName(nodeName string) *handlers.NodeConfig {
	return n.NodeConfigs[nodeName]
}

func (n *Manager) Apply(nodeName string) error {
	return nil
}

func (n *Manager) GetConfigsString() string {
	b, err := json.Marshal(n.NodeConfigs)
	if err != nil {
		klog.Errorf("cannot convert nodeConfig map to json string")
	}
	return string(b)
}

func (n *Manager) SortConfigs() {
	if len(n.NodeConfigs) == 0 {
		return
	}

	for _, c := range n.NodeConfigs {
		c.Sort()
	}
}
