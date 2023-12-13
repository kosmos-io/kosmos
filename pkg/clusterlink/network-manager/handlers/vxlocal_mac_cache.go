package handlers

import (
	"fmt"
	"net"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/constants"
)

type VxLocalMacCache struct {
	Next
}

func (h *VxLocalMacCache) Do(c *Context) (err error) {
	// add internal => gw cache
	nodes := c.Filter.GetInternalNodes()
	for _, node := range nodes {
		ipTypes := h.getSupportIPTypes(node, c)
		gw := c.Filter.GetGatewayNodeByClusterName(node.Spec.ClusterName)
		if gw == nil {
			klog.Warning("cannot find gateway node, cluster name: %s", node.Spec.ClusterName)
			continue
		}

		for _, ipType := range ipTypes {
			fdb, arp, err := h.buildVxLocalCachesByNode(c, ipType, gw)
			if err != nil {
				klog.Errorf("VxLocalMacCache, build vxLocal caches with err : %v", err)
				continue
			}

			c.Results[node.Name].Fdbs = append(c.Results[node.Name].Fdbs, fdb...)
			c.Results[node.Name].Arps = append(c.Results[node.Name].Arps, arp...)
		}
	}

	// add gw => internal cache for host-network
	gwNodes := c.Filter.GetGatewayNodes()
	for _, src := range gwNodes {
		var fdb []v1alpha1.Fdb
		var arp []v1alpha1.Arp

		ipTypes := h.getSupportIPTypes(src, c)
		internalNodes := c.Filter.GetInternalNodesByClusterName(src.Spec.ClusterName)
		for _, tar := range internalNodes {
			for _, ipType := range ipTypes {
				f, a, err := h.buildVxLocalCachesByNode(c, ipType, tar)
				if err != nil {
					klog.Errorf("VxLocalMacCache, build vxLocal caches with err : %v, node: %s", err, tar.Name)
					continue
				}

				fdb = append(fdb, f...)
				arp = append(arp, a...)
			}
		}

		c.Results[src.Name].Fdbs = append(c.Results[src.Name].Fdbs, fdb...)
		c.Results[src.Name].Arps = append(c.Results[src.Name].Arps, arp...)
	}

	return
}

func (h *VxLocalMacCache) getSupportIPTypes(node *v1alpha1.ClusterNode, c *Context) []v1alpha1.IPFamilyType {
	var ipTypes []v1alpha1.IPFamilyType
	supportIPv4 := c.Filter.SupportIPv4(node)
	supportIPv6 := c.Filter.SupportIPv6(node)
	if supportIPv4 {
		ipTypes = append(ipTypes, v1alpha1.IPFamilyTypeIPV4)
	}
	if supportIPv6 {
		ipTypes = append(ipTypes, v1alpha1.IPFamilyTypeIPV6)
	}
	return ipTypes
}

func (h *VxLocalMacCache) buildVxLocalCachesByNode(c *Context, ipType v1alpha1.IPFamilyType, node *v1alpha1.ClusterNode) ([]v1alpha1.Fdb, []v1alpha1.Arp, error) {
	var fdb []v1alpha1.Fdb
	var arp []v1alpha1.Arp

	var devName string
	var hostIP string

	if ipType == v1alpha1.IPFamilyTypeIPV4 {
		devName = constants.VXLAN_LOCAL_NAME
		hostIP = node.Spec.IP
	} else if ipType == v1alpha1.IPFamilyTypeIPV6 {
		devName = constants.VXLAN_LOCAL_NAME_6
		hostIP = node.Spec.IP6
	}

	dev := c.GetDeviceFromResults(node.Name, devName)
	if dev == nil {
		return fdb, arp, fmt.Errorf("device not found devName: %s, nodeName: %s", devName, node.Name)
	}

	// 00:00:00:00:00:00 dev vx-local dst <remote-host-ip> self permanent
	// <remote-vxLocal-mac> dev vx-local dst <remote-host-ip> self permanent
	fdb = append(fdb, []v1alpha1.Fdb{
		{
			IP:  hostIP,
			Mac: constants.ALL_ZERO_MAC,
			Dev: devName,
		},
		{
			IP:  hostIP,
			Mac: dev.Mac,
			Dev: devName,
		},
	}...)

	devIP, _, err := net.ParseCIDR(dev.Addr)
	if err != nil {
		return fdb, arp, fmt.Errorf("cannot parse dev addr, nodeName: %s, devName: %s", node.Name, dev.Name)
	}

	// arp -s <remote-vxLocal-ip> <remote-vxLocal-mac> -i <local-dev>
	arp = append(arp, v1alpha1.Arp{
		IP:  devIP.String(),
		Mac: dev.Mac,
		Dev: devName,
	})

	return fdb, arp, nil
}
