package utils

const (
	// NamespaceClusterLinksystem is the clusterlink system namespace.
	NamespaceClusterLinksystem = "clusterlink-system"
)

const ExternalIPPoolNamePrefix = "clusterlink"

const CNITypeCalico = "calico"

const (
	ProxySecretName        = "clusterlink-agent-proxy"
	ControlPanelSecretName = "controlpanel-config"
)

const (
	EnvUseProxy    = "USE_PROXY"
	EnvClusterName = "CLUSTER_NAME"
	EnvNodeName    = "NODE_NAME"
)

const ClusterStartControllerFinalizer = "kosmos.io/cluster-start-finazlizer"
