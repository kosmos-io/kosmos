package utils

const (
	DefaultNamespace       = "kosmos-system"
	DefaultImageRepository = "ghcr.io/kosmos-io"
	DefaultInstallModule   = "all"
)

const ExternalIPPoolNamePrefix = "clusterlink"

const (
	CNITypeCalico  = "calico"
	NetworkTypeP2P = "p2p"
)

const (
	ProxySecretName        = "clusterlink-agent-proxy"
	ControlPanelSecretName = "controlpanel-config"
	HostKubeConfigName     = "host-kubeconfig"
)

const (
	EnvUseProxy    = "USE_PROXY"
	EnvClusterName = "CLUSTER_NAME"
	EnvNodeName    = "NODE_NAME"
)

const ClusterStartControllerFinalizer = "kosmos.io/cluster-start-finazlizer"
