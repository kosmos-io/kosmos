package utils

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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

// mcs consts
const (
	ServiceExportControllerName = "service-export-controller"
	ServiceKey                  = "kubernetes.io/service-name"
	ServiceImportControllerName = "serviceimport-controller"
	ServiceExportLabelKey       = "kosmos.io/service-export"
	ServiceImportLabelKey       = "kosmos.io/service-import"
	MCSLabelValue               = "ture"
	ServiceEndpointsKey         = "kosmos.io/address"
	DisconnectedEndpointsKey    = "kosmos.io/disconnected-address"
)

// cluster node
const (
	KosmosNodeLabel        = "kosmos.io/node"
	KosmosNodeValue        = "true"
	KosmosNodeTaintKey     = "kosmos.io/node"
	KosmosNodeTaintValue   = "true"
	KosmosNodeTaintEffect  = "NoSchedule"
	KosmosPodLabel         = "kosmos-io/pod"
	KosmosGlobalLabel      = "kosmos.io/global"
	KosmosSelectorKey      = "kosmos.io/cluster-selector"
	KosmosTrippedLabels    = "kosmos-io/tripped"
	KosmosPvcLabelSelector = "kosmos-io/label-selector"

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
