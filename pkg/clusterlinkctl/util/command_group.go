package util

const (
	// TagCommandGroup used for tag the group of the command
	TagCommandGroup = "commandGroup"
)

const (
	// GroupBasic means the command belongs to Group "Basic Commands"
	GroupBasic = "Basic Commands"

	// GroupClusterRegistration means the command belongs to Group "Cluster Registration Commands"
	GroupClusterRegistration = "Cluster Registration Commands"

	// GroupClusterManagement means the command belongs to Group "Cluster Management Commands"
	GroupClusterManagement = "Cluster Management Commands"

	// GroupClusterTroubleshootingAndDebugging means the command belongs to Group "Troubleshooting and Debugging Commands"
	GroupClusterTroubleshootingAndDebugging = "Troubleshooting and Debugging Commands"

	// GroupAdvancedCommands means the command belongs to Group "Advanced Commands"
	GroupAdvancedCommands = "Advanced Commands"
)

type Protocol string

const (
	TCP  Protocol = "tcp"
	UDP  Protocol = "udp"
	IPv4 Protocol = "ipv4"
)
