package handlers

import (
	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	constants "github.com/kosmos.io/clusterlink/pkg/network"
	"github.com/kosmos.io/clusterlink/pkg/network-manager/helpers"
)

type VxBridgeNetwork struct {
	Next
}

func (h *VxBridgeNetwork) Do(c *Context) (err error) {

	gwNodes := c.Filter.GetGatewayNodes()
	epNodes := c.Filter.GetEndpointNodes()

	nodes := append(gwNodes, epNodes...)

	for _, node := range nodes {
		cluster := c.Filter.GetClusterByName(node.Spec.ClusterName)

		if h.needToCreateVxBridge(c, node, cluster) {
			dev := h.createVxBridge(c, node, cluster)
			if dev != nil {
				c.Results[node.Name].Devices = append(c.Results[node.Name].Devices, *dev)
			}
		}

		if h.needToCreateVxBridge6(c, node, cluster) {
			dev := h.createVxBridge6(c, node, cluster)
			if dev != nil {
				c.Results[node.Name].Devices = append(c.Results[node.Name].Devices, *dev)
			}
		}
	}

	return
}

func (h *VxBridgeNetwork) needToCreateVxBridge(c *Context, clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster) bool {
	return c.Filter.SupportIPv4(clusterNode) &&
		clusterNode.Spec.IP != "" &&
		cluster.Spec.BridgeCIDRs.IP != ""
}

func (h *VxBridgeNetwork) needToCreateVxBridge6(c *Context, clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster) bool {
	return c.Filter.SupportIPv6(clusterNode) &&
		clusterNode.Spec.IP6 != "" &&
		cluster.Spec.BridgeCIDRs.IP6 != ""
}

func (h *VxBridgeNetwork) createVxBridge(c *Context, clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster) *v1alpha1.Device {
	devOld := c.Filter.GetDeviceFromNodeConfig(clusterNode.Name, constants.VXLAN_BRIDGE_NAME)
	dev := helpers.BuildVxlanDevice(constants.VXLAN_BRIDGE_NAME, clusterNode.Spec.IP, cluster.Spec.BridgeCIDRs.IP, clusterNode.Spec.InterfaceName)
	if devOld != nil && devOld.Mac != "" {
		dev.Mac = devOld.Mac
	}
	return dev
}

func (h *VxBridgeNetwork) createVxBridge6(c *Context, clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster) *v1alpha1.Device {
	devOld := c.Filter.GetDeviceFromNodeConfig(clusterNode.Name, constants.VXLAN_BRIDGE_NAME_6)
	dev := helpers.BuildVxlanDevice(constants.VXLAN_BRIDGE_NAME_6, clusterNode.Spec.IP6, cluster.Spec.BridgeCIDRs.IP6, clusterNode.Spec.InterfaceName)
	if devOld != nil && devOld.Mac != "" {
		dev.Mac = devOld.Mac
	}
	return dev
}
