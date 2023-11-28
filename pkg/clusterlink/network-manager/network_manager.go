package network_manager

import (
	"encoding/json"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network-manager/handlers"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network-manager/helpers"
)

type Manager struct {
	NodeConfigs map[string]*handlers.NodeConfig
}

func NewManager() *Manager {
	return &Manager{}
}

// ExcludeInvalidItems Verify whether clusterNodes and clusters are valid and give instructions
func ExcludeInvalidItems(clusters []v1alpha1.Cluster, clusterNodes []v1alpha1.ClusterNode) (cs []v1alpha1.Cluster, cns []v1alpha1.ClusterNode) {
	klog.Infof("Start verifying clusterNodes and clusters")
	clustersMap := map[string]v1alpha1.Cluster{}
	for _, c := range clusters {
		if c.Spec.ClusterLinkOptions == nil {
			klog.Infof("the cluster %s's ClusterLinkOptions is empty, will exclude.", c.Name)
			continue
		}
		clustersMap[c.Name] = c
		cs = append(cs, c)
	}

	for _, cn := range clusterNodes {
		if len(cn.Spec.ClusterName) == 0 {
			klog.Infof("the clusterNode %s's clusterName is empty, will exclude.", cn.Name)
			continue
		}
		if len(cn.Spec.InterfaceName) == 0 {
			klog.Infof("the clusterNode %s's interfaceName is empty, will exclude.", cn.Name)
			continue
		}

		if _, ok := clustersMap[cn.Spec.ClusterName]; !ok {
			klog.Infof("the cluster which clusterNode %s belongs to does not exist, or the cluster lacks the spec.clusterLinkOptions configuration.", cn.Name)
			continue
		}

		c := clustersMap[cn.Spec.ClusterName]
		supportIPv4 := c.Spec.ClusterLinkOptions.IPFamily == v1alpha1.IPFamilyTypeALL || c.Spec.ClusterLinkOptions.IPFamily == v1alpha1.IPFamilyTypeIPV4
		supportIPv6 := c.Spec.ClusterLinkOptions.IPFamily == v1alpha1.IPFamilyTypeALL || c.Spec.ClusterLinkOptions.IPFamily == v1alpha1.IPFamilyTypeIPV6
		if supportIPv4 && len(cn.Spec.IP) == 0 {
			klog.Infof("the clusterNode %s's ip is empty, but cluster's ipFamily is %s", cn.Name, c.Spec.ClusterLinkOptions.IPFamily)
			continue
		}
		if supportIPv6 && len(cn.Spec.IP6) == 0 {
			klog.Infof("the clusterNode %s's ip6 is empty, but cluster's ipFamily is %s", cn.Name, c.Spec.ClusterLinkOptions.IPFamily)
			continue
		}

		cns = append(cns, cn)
	}
	return
}

// CalculateNetworkConfigs Calculate the network configuration required for each node
func (n *Manager) CalculateNetworkConfigs(clusters []v1alpha1.Cluster, clusterNodes []v1alpha1.ClusterNode, nodeConfigs []v1alpha1.NodeConfig) (map[string]*handlers.NodeConfig, error) {
	cs, cns := ExcludeInvalidItems(clusters, clusterNodes)
	filter := helpers.NewFilter(cs, cns, nodeConfigs)

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
		SetNext(&handlers.HostNetwork{}).
		SetNext(&handlers.GlobalMap{})

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
