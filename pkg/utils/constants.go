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
	Containerd                   = "containerd"
	DefaultContainerRuntime      = "docker"
	DefaultContainerdNamespace   = "default"
	DefaultContainerdSockAddress = "/run/containerd/containerd.sock"
	DefaultVersion               = "latest"
	DefaultTarName               = "kosmos-io.tar.gz"
	// nolint
	DefaultServiceAccountName = "default"
	// nolint
	DefaultServiceAccountToken = "kosmos.io/service-account.name"
)

const (
	DefaultNamespace           = "kosmos-system"
	DefaultClusterName         = "kosmos-control-cluster"
	DefaultImageRepository     = "ghcr.io/kosmos-io"
	DefaultImageVersion        = "v1.21.5-eki.0"
	DefaultCoreDNSImageTag     = "v1.9.3"
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
	Scheduler   = "scheduler"
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
	OperatorName           = "clusterlink-operator"
	ControlPanelSecretName = "controlpanel-config"
	HostKubeConfigName     = "host-kubeconfig"
	NodeConfigFile         = "~/nodeconfig.json"
)

const (
	EnvUseProxy    = "USE_PROXY"
	EnvClusterName = "CLUSTER_NAME"
	EnvNodeName    = "NODE_NAME"
)

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
	KosmosNodePrefix        = "kosmos-"
	KosmosNodeLabel         = "kosmos.io/node"
	KosmosActualClusterName = "kosmos.io/actual-cluster-name"
	KosmosNodeValue         = "true"
	KosmosNodeJoinLabel     = "kosmos.io/join"
	KosmosNodeJoinValue     = "true"
	KosmosNodeTaintKey      = "kosmos.io/node"
	KosmosNodeTaintValue    = "true"
	KosmosNodeTaintEffect   = "NoSchedule"
	KosmosPodLabel          = "kosmos-io/pod"
	KosmosGlobalLabel       = "kosmos.io/global"
	KosmosSelectorKey       = "kosmos.io/cluster-selector"
	KosmosTrippedLabels     = "kosmos-io/tripped"
	KosmosConvertLabels     = "kosmos-io/convert-policy"
	KosmosPvcLabelSelector  = "kosmos-io/label-selector"
	KosmosExcludeNodeLabel  = "kosmos.io/exclude"
	KosmosExcludeNodeValue  = "true"

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
	OpenebsPVNodeLabel    = "openebs.io/nodename"
	NodeOSLabelStable     = corev1.LabelOSStable
	NodeArchLabelStable   = corev1.LabelArchStable
	PVCSelectedNodeKey    = "volume.kubernetes.io/selected-node"

	DefaultK8sOS   = "linux"
	DefaultK8sArch = "amd64"

	DefaultInformerResyncPeriod = 0

	DefaultLeafKubeQPS   = 40.0
	DefaultLeafKubeBurst = 60

	// LabelNodeRoleControlPlane specifies that a node hosts control-plane components
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"

	// LabelNodeRoleOldControlPlane specifies that a node hosts control-plane components
	LabelNodeRoleOldControlPlane = "node-role.kubernetes.io/master"

	// LabelNodeRoleNode specifies that a node hosts node components
	LabelNodeRoleNode = "node-role.kubernetes.io/node"

	DefaultRequeueTime = 10 * time.Second
)

const (
	ReservedNS          = "kube-system"
	RooTCAConfigMapName = "kube-root-ca.crt"
	SATokenPrefix       = "kube-api-access"
	MasterRooTCAName    = "master-root-ca.crt"
)

// finalizers
const (
	ClusterStartControllerFinalizer = "kosmos.io/cluster-start-finalizer"
	MCSFinalizer                    = "kosmos.io/multi-cluster-service-finalizer"
)

// nolint:revive
var GVR_CONFIGMAP = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "configmaps",
}

// nolint:revive
var GVR_PVC = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "persistentvolumeclaims",
}

// nolint:revive
var GVR_SECRET = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "secrets",
}

// nolint:revive
var GVR_SERVICE = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "services",
}
