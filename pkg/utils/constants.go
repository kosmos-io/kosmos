package utils

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ImageList = []string{
		ClusterTreeClusterManager,
		ClusterLinkOperator,
		ClusterLinkAgent,
		ClusterLinkNetworkManager,
		ClusterLinkControllerManager,
		ClusterLinkProxy,
		ClusterLinkElector,
		ClusterLinkFloater,
		KosmosOperator,
		Coredns,
		EpsProbePlugin}
)

const (
	ClusterTreeClusterManager    = "ghcr.io/kosmos-io/clustertree-cluster-manager"
	ClusterLinkOperator          = "ghcr.io/kosmos-io/clusterlink-operator"
	ClusterLinkAgent             = "ghcr.io/kosmos-io/clusterlink-agent"
	ClusterLinkNetworkManager    = "ghcr.io/kosmos-io/clusterlink-network-manager"
	ClusterLinkControllerManager = "ghcr.io/kosmos-io/clusterlink-controller-manager"
	ClusterLinkProxy             = "ghcr.io/kosmos-io/clusterlink-proxy"
	ClusterLinkElector           = "ghcr.io/kosmos-io/clusterlink-elector"
	ClusterLinkFloater           = "ghcr.io/kosmos-io/clusterlink-floater"
	Coredns                      = "ghcr.io/kosmos-io/coredns"
	EpsProbePlugin               = "ghcr.io/kosmos-io/eps-probe-plugin"
	KosmosOperator               = "ghcr.io/kosmos-io/kosmos-operator"
	Containerd                   = "containerd"
	DefaultContainerRuntime      = "docker"
	DefaultContainerdNamespace   = "default"
	DefaultContainerdSockAddress = "/run/containerd/containerd.sock"
	DefaultVersion               = "latest"
	DefaultTarName               = "kosmos-io.tar.gz"
)

const (
	DefaultNamespace           = "kosmos-system"
	DefaultClusterName         = "kosmos-control-cluster"
	DefaultImageRepository     = "ghcr.io/kosmos-io"
	DefaultWaitTime            = 120
	RootClusterAnnotationKey   = "kosmos.io/cluster-role"
	RootClusterAnnotationValue = "root"
	KosmosSchedulerName        = "kosmos-scheduler"
)

const (
	All         = "all"
	ClusterLink = "clusterlink"
	ClusterTree = "clustertree"
	CoreDNS     = "coredns"
)

const ExternalIPPoolNamePrefix = "clusterlink"

const (
	CNITypeCalico      = "calico"
	NetworkTypeP2P     = "p2p"
	NetworkTypeGateway = "gateway"
	DefaultIPv4        = "ipv4"
	DefaultIPv6        = "ipv6"
	DefaultPort        = "8889"
)

const (
	ProxySecretName        = "clusterlink-agent-proxy"
	ControlPanelSecretName = "controlpanel-config"
	HostKubeConfigName     = "host-kubeconfig"
	NodeConfigFile         = "~/nodeconfig.json"
)

const (
	EnvUseProxy    = "USE_PROXY"
	EnvClusterName = "CLUSTER_NAME"
	EnvNodeName    = "NODE_NAME"
)

const ClusterStartControllerFinalizer = "kosmos.io/cluster-start-finazlizer"

// mcs
const (
	ServiceKey               = "kubernetes.io/service-name"
	ServiceExportLabelKey    = "kosmos.io/service-export"
	ServiceImportLabelKey    = "kosmos.io/service-import"
	MCSLabelValue            = "ture"
	ServiceEndpointsKey      = "kosmos.io/address"
	DisconnectedEndpointsKey = "kosmos.io/disconnected-address"
	AutoCreateMCSAnnotation  = "kosmos.io/auto-create-mcs"
)

// cluster node
const (
	KosmosNodePrefix       = "kosmos-"
	KosmosNodeLabel        = "kosmos.io/node"
	KosmosNodeValue        = "true"
	KosmosNodeJoinLabel    = "kosmos.io/join"
	KosmosNodeJoinValue    = "true"
	KosmosNodeTaintKey     = "kosmos.io/node"
	KosmosNodeTaintValue   = "true"
	KosmosNodeTaintEffect  = "NoSchedule"
	KosmosPodLabel         = "kosmos-io/pod"
	KosmosGlobalLabel      = "kosmos.io/global"
	KosmosSelectorKey      = "kosmos.io/cluster-selector"
	KosmosTrippedLabels    = "kosmos-io/tripped"
	KosmosPvcLabelSelector = "kosmos-io/label-selector"
	KosmosExcludeNodeLabel = "kosmos.io/exclude"
	KosmosExcludeNodeValue = "true"

	// on resorce (pv, configmap, secret), represents which cluster this resource belongs to
	KosmosResourceOwnersAnnotations = "kosmos-io/cluster-owners"
	// on node, represents which cluster this node belongs to
	KosmosNodeOwnedByClusterAnnotations = "kosmos-io/owned-by-cluster"

	KosmosDaemonsetAllowAnnotations = "kosmos-io/daemonset-allow"

	NodeRoleLabel         = "kubernetes.io/role"
	NodeRoleValue         = "agent"
	NodeOSLabelBeta       = "beta.kubernetes.io/os"
	NodeHostnameValue     = corev1.LabelHostname
	NodeHostnameValueBeta = "beta.kubernetes.io/hostname"
	NodeOSLabelStable     = corev1.LabelOSStable
	NodeArchLabelStable   = corev1.LabelArchStable
	PVCSelectedNodeKey    = "volume.kubernetes.io/selected-node"

	DefaultK8sOS   = "linux"
	DefaultK8sArch = "amd64"

	DefaultInformerResyncPeriod = 1 * time.Minute
	DefaultListenPort           = 10250
	DefaultPodSyncWorkers       = 10
	DefaultWorkers              = 5
	DefaultKubeNamespace        = corev1.NamespaceAll

	DefaultTaintEffect = string(corev1.TaintEffectNoSchedule)
	DefaultTaintKey    = "kosmos-node.io/plugin"

	DefaultLeafKubeQPS   = 40.0
	DefaultLeafKubeBurst = 60

	// LabelNodeRoleControlPlane specifies that a node hosts control-plane components
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"

	// LabelNodeRoleOldControlPlane specifies that a node hosts control-plane components
	LabelNodeRoleOldControlPlane = "node-role.kubernetes.io/master"

	// LabelNodeRoleNode specifies that a node hosts node components
	LabelNodeRoleNode = "node-role.kubernetes.io/node"
)

const (
	ReservedNS          = "kube-system"
	RooTCAConfigMapName = "kube-root-ca.crt"
	SATokenPrefix       = "kube-api-access"
	MasterRooTCAName    = "master-root-ca.crt"
)

var GVR_CONFIGMAP = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "configmaps",
}

var GVR_PVC = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "persistentvolumeclaims",
}

var GVR_SECRET = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "secrets",
}

var GVR_SERVICE = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "services",
}
