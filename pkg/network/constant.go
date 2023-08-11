package network

import (
	"net"

	"github.com/vishvananda/netlink"
)

const vxlanOverhead = 50

const AutoSelectInterfaceFlag = "*"

type VxlanType int

const (
	BRIDGE VxlanType = 0
	LOCAL  VxlanType = 1
)

// type IPFamilySupport string

// const (
// 	IPFamilyTypeALL  IPFamilySupport = "0"
// 	IPFamilyTypeIPV4 IPFamilySupport = "1"
// 	IPFamilyTypeIPV6 IPFamilySupport = "2"
// )

// IPFamilyTypeALL =

const (
	TABLE_ID = 200

	VXLAN_BRIDGE_NAME = "vx-bridge"
	VXLAN_LOCAL_NAME  = "vx-local"

	VXLAN_BRIDGE_NAME_6 = "vx-bridge-6"
	VXLAN_LOCAL_NAME_6  = "vx-local-6"

	VXLAN_OVERHEAD = 50

	ALL_ADDR_IPV4 = "0.0.0.0/0"
	ALL_ADDR_IPV6 = "0.0.0.0.0.0.0.0/0"

	// VXLAN_ADDR_PREFIX_BRIDGE_IPv4 int = 220
	// VXLAN_ADDR_PREFIX_LOCAL_IPv4  int = 210

	// VXLAN_ADDR_PREFIX_BRIDGE_IPv6 int = 9480 // 0x2508
	// VXLAN_ADDR_PREFIX_LOCAL_IPv6  int = 9470 // 0x24fe

	VXLAN_BRIDGE_NET_IPV4 = "220.0.0.0/8"
	VXLAN_BRIDGE_NET_IPV6 = "9480::0/16"

	VXLAN_LOCAL_NET_IPV4 = "210.0.0.0/8"
	VXLAN_LOCAL_NET_IPV6 = "9470::0/16"

	VXLAN_BRIDGE_ID   = 54
	VXLAN_BRIDGE_PORT = 4876

	VXLAN_LOCAL_ID   = 55
	VXLAN_LOCAL_PORT = 4877

	VXLAN_BRIDGE_ID_6   = 64
	VXLAN_BRIDGE_PORT_6 = 4866

	VXLAN_LOCAL_ID_6   = 65
	VXLAN_LOCAL_PORT_6 = 4867

	ALL_ZERO_MAC = "00:00:00:00:00:00"

	ClusterLinkPreRoutingChain  = "CLUSTERLINK-PREROUTING"
	ClusterLinkPostRoutingChain = "CLUSTERLINK-POSTROUTING"

	IPTablesPreRoutingChain  = "PREROUTING"
	IPTablesPostRoutingChain = "POSTROUTING"
)

type vxlanAttributes struct {
	name     string
	vxlanID  int
	group    net.IP
	srcAddr  net.IP
	vtepPort int
	overhead int
	cidr     string
	family   int
}

var VXLAN_BRIDGE = &vxlanAttributes{
	name:     VXLAN_BRIDGE_NAME,
	vxlanID:  VXLAN_BRIDGE_ID,
	group:    nil,
	srcAddr:  nil,
	vtepPort: VXLAN_BRIDGE_PORT,
	overhead: vxlanOverhead,
	cidr:     VXLAN_BRIDGE_NET_IPV4,
	family:   netlink.FAMILY_V4,
}

var VXLAN_LOCAL = &vxlanAttributes{
	name:     VXLAN_LOCAL_NAME,
	vxlanID:  VXLAN_LOCAL_ID,
	group:    nil,
	srcAddr:  nil,
	vtepPort: VXLAN_LOCAL_PORT,
	overhead: vxlanOverhead,
	cidr:     VXLAN_LOCAL_NET_IPV4,
	family:   netlink.FAMILY_V4,
}

var VXLAN_BRIDGE_6 = &vxlanAttributes{
	name:     VXLAN_BRIDGE_NAME_6,
	vxlanID:  VXLAN_BRIDGE_ID_6,
	group:    nil,
	srcAddr:  nil,
	vtepPort: VXLAN_BRIDGE_PORT_6,
	overhead: vxlanOverhead,
	cidr:     VXLAN_BRIDGE_NET_IPV6,
	family:   netlink.FAMILY_V6,
}

var VXLAN_LOCAL_6 = &vxlanAttributes{
	name:     VXLAN_LOCAL_NAME_6,
	vxlanID:  VXLAN_LOCAL_ID_6,
	group:    nil,
	srcAddr:  nil,
	vtepPort: VXLAN_LOCAL_PORT_6,
	overhead: vxlanOverhead,
	cidr:     VXLAN_LOCAL_NET_IPV6,
	family:   netlink.FAMILY_V6,
}

var ALL_DEVICES = []*vxlanAttributes{VXLAN_BRIDGE, VXLAN_LOCAL, VXLAN_BRIDGE_6, VXLAN_LOCAL_6}
