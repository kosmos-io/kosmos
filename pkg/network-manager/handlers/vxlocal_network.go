package handlers

import (
	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	constants "github.com/kosmos.io/clusterlink/pkg/network"
	"github.com/kosmos.io/clusterlink/pkg/network-manager/helpers"
)

type VxLocalNetwork struct {
	Next
}

func (h *VxLocalNetwork) Do(c *Context) (err error) {

	nodes := c.Filter.GetGatewayClusterNodes()

	for _, node := range nodes {
		cluster := c.Filter.GetClusterByName(node.Spec.ClusterName)

		if h.needToCreateVxLocal(c, node, cluster) {
			dev := h.createVxLocal(c, node, cluster)
			if dev != nil {
				c.Results[node.Name].Devices = append(c.Results[node.Name].Devices, *dev)
			}
		}

		if h.needToCreateVxLocal6(c, node, cluster) {
			dev := h.createVxLocal6(c, node, cluster)
			if dev != nil {
				c.Results[node.Name].Devices = append(c.Results[node.Name].Devices, *dev)
			}
		}
	}

	return
}

func (h *VxLocalNetwork) needToCreateVxLocal(c *Context, clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster) bool {
	return c.Filter.SupportIPv4(clusterNode) &&
		clusterNode.Spec.IP != "" &&
		cluster.Spec.LocalCIDRs.IP != ""
}

func (h *VxLocalNetwork) needToCreateVxLocal6(c *Context, clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster) bool {
	return c.Filter.SupportIPv6(clusterNode) &&
		clusterNode.Spec.IP6 != "" &&
		cluster.Spec.LocalCIDRs.IP6 != ""
}

func (h *VxLocalNetwork) createVxLocal(c *Context, clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster) *v1alpha1.Device {
	devOld := c.Filter.GetDeviceFromNodeConfig(clusterNode.Name, constants.VXLAN_LOCAL_NAME)
	dev := helpers.BuildVxlanDevice(constants.VXLAN_LOCAL_NAME, clusterNode.Spec.IP, cluster.Spec.LocalCIDRs.IP, clusterNode.Spec.InterfaceName)
	if devOld != nil && devOld.Mac != "" {
		dev.Mac = devOld.Mac
	}
	return dev
}

func (h *VxLocalNetwork) createVxLocal6(c *Context, clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster) *v1alpha1.Device {
	devOld := c.Filter.GetDeviceFromNodeConfig(clusterNode.Name, constants.VXLAN_LOCAL_NAME_6)
	dev := helpers.BuildVxlanDevice(constants.VXLAN_LOCAL_NAME_6, clusterNode.Spec.IP6, cluster.Spec.LocalCIDRs.IP6, clusterNode.Spec.InterfaceName)
	if devOld != nil && devOld.Mac != "" {
		dev.Mac = devOld.Mac
	}
	return dev
}
