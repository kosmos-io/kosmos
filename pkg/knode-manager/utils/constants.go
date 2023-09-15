package utils

import corev1 "k8s.io/api/core/v1"

const (
	KosmosNodeLabel       = "kosmos.io/node"
	KosmosNodeValue       = "true"
	KosmosNodeTaintKey    = "kosmos.io/node"
	KosmosNodeTaintValue  = "true"
	KosmosNodeTaintEffect = "NoSchedule"
	KosmosPodLabel        = "kosmos-io/pod"
	KosmosGlobalLabel     = "kosmos.io/global"
	KosmosSelectorKey     = "kosmos.io/cluster-selector"
	KosmosTrippedLabels   = "kosmos-io/tripped"

	NodeRoleLabel       = "kubernetes.io/role"
	NodeRoleValue       = "agent"
	NodeOSLabelBeta     = "beta.kubernetes.io/os"
	NodeHostnameValue   = corev1.LabelHostname
	NodeOSLabelStable   = corev1.LabelOSStable
	NodeArchLabelStable = corev1.LabelArchStable

	DefaultK8sOS   = "linux"
	DefaultK8sArch = "amd64"
)
