package constants

type VxlanType int

const (
	VXLAN_BRIDGE_NAME = "vx-bridge"
	VXLAN_LOCAL_NAME  = "vx-local"

	VXLAN_BRIDGE_NAME_6 = "vx-bridge-6"
	VXLAN_LOCAL_NAME_6  = "vx-local-6"

	VXLAN_BRIDGE_ID   = 54
	VXLAN_BRIDGE_PORT = 4876

	VXLAN_LOCAL_ID   = 55
	VXLAN_LOCAL_PORT = 4877

	VXLAN_BRIDGE_ID_6   = 64
	VXLAN_BRIDGE_PORT_6 = 4866

	VXLAN_LOCAL_ID_6   = 65
	VXLAN_LOCAL_PORT_6 = 4867

	ALL_ZERO_MAC = "00:00:00:00:00:00"

	IPTablesPreRoutingChain  = "PREROUTING"
	IPTablesPostRoutingChain = "POSTROUTING"
)
