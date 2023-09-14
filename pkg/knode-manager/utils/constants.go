package utils

import corev1 "k8s.io/api/core/v1"

const (
	KosmosNodeLabel = "kosmos.io/node"
	KosmosNodeValue = "true"

	NodeRoleLabel       = "kubernetes.io/role"
	NodeRoleValue       = "agent"
	NodeOSLabelBeta     = "beta.kubernetes.io/os"
	NodeHostnameValue   = corev1.LabelHostname
	NodeOSLabelStable   = corev1.LabelOSStable
	NodeArchLabelStable = corev1.LabelArchStable

	DefaultK8sOS   = "linux"
	DefaultK8sArch = "amd64"
)
