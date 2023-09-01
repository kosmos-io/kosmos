package v1alpha1

type NetworkType string

const (
	NetworkTypeP2P     NetworkType = "p2p"
	NetWorkTypeGateWay NetworkType = "gateway"
)

type IPFamilyType string

const (
	IPFamilyTypeALL  IPFamilyType = "all"
	IPFamilyTypeIPV4 IPFamilyType = "ipv4"
	IPFamilyTypeIPV6 IPFamilyType = "ipv6"
)

type Role string

const (
	// RoleGateway
	// Nodes with this role serve as the entry point for service traffic.
	RoleGateway Role = "gateway"
)

type DeviceType string

const (
	VxlanDevice DeviceType = "vxlan"
)
