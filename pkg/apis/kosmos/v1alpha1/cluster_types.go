package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="NETWORK_TYPE",type=string,JSONPath=`.spec.clusterLinkOptions.networkType`
// +kubebuilder:printcolumn:name="IP_FAMILY",type=string,JSONPath=`.spec.clusterLinkOptions.ipFamily`

type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the cluster.
	Spec ClusterSpec `json:"spec"`

	// Status describes the current status of a cluster.
	// +optional
	Status ClusterStatus `json:"status"`
}

type ClusterSpec struct {
	// +optional
	Kubeconfig []byte `json:"kubeconfig,omitempty"`

	// +kubebuilder:default=kosmos-system
	// +optional
	Namespace string `json:"namespace"`

	// +optional
	ImageRepository string `json:"imageRepository,omitempty"`

	// +optional
	ClusterLinkOptions *ClusterLinkOptions `json:"clusterLinkOptions,omitempty"`

	// +optional
	ClusterTreeOptions *ClusterTreeOptions `json:"clusterTreeOptions,omitempty"`
}

type ClusterStatus struct {
	// ClusterLinkStatus contain the cluster network information
	// +optional
	ClusterLinkStatus ClusterLinkStatus `json:"clusterLinkStatus,omitempty"`

	// ClusterTreeStatus contain the member cluster leafNode end status
	// +optional
	ClusterTreeStatus ClusterTreeStatus `json:"clusterTreeStatus,omitempty"`
}

type ClusterLinkOptions struct {
	// +kubebuilder:default=true
	// +optional
	Enable bool `json:"enable"`

	// +kubebuilder:default=calico
	// +optional
	CNI string `json:"cni"`

	// +kubebuilder:validation:Enum=p2p;gateway
	// +kubebuilder:default=p2p
	// +optional
	NetworkType NetworkType `json:"networkType"`

	// +kubebuilder:default=all
	// +optional
	IPFamily IPFamilyType `json:"ipFamily"`

	// +kubebuilder:default=false
	// +optional
	UseIPPool bool `json:"useIPPool,omitempty"`

	// +kubebuilder:default={ip:"210.0.0.0/8",ip6:"9480::/16"}
	// +optional
	LocalCIDRs VxlanCIDRs `json:"localCIDRs,omitempty"`

	// +kubebuilder:default={ip:"220.0.0.0/8",ip6:"9470::/16"}
	// +optional
	BridgeCIDRs VxlanCIDRs `json:"bridgeCIDRs,omitempty"`

	// +optional
	NICNodeNames []NICNodeNames `json:"nicNodeNames,omitempty"`

	// +kubebuilder:default=*
	// +optional
	DefaultNICName string `json:"defaultNICName,omitempty"`

	// +optional
	GlobalCIDRsMap map[string]string `json:"globalCIDRsMap,omitempty"`

	// +optional
	AutodetectionMethod string `json:"autodetectionMethod,omitempty"`
}

type ClusterTreeOptions struct {
	// +kubebuilder:default=true
	// +optional
	Enable bool `json:"enable"`

	// LeafModels provide an api to arrange the member cluster with some rules to pretend one or more leaf node
	// +optional
	LeafModels []LeafModel `json:"leafModels,omitempty"`

	// +kubebuilder:default="k8s"
	// +optional
	LeafType string `json:"leafType,omitempty"`

	// secret?
	// +optional
	AccessKey string `json:"accessKey,omitempty"`

	// +optional
	SecretKey string `json:"secretKey,omitempty"`
}

type LeafModel struct {
	// LeafNodeName defines leaf name
	// If nil or empty, the leaf node name will generate by controller and fill in cluster link status
	// +optional
	LeafNodeName string `json:"leafNodeName,omitempty"`

	// Labels that will be setting in the pretended Node labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Taints attached to the leaf pretended Node.
	// If nil or empty, controller will set the default no-schedule taint
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`

	// NodeSelector is a selector to select member cluster nodes to pretend a leaf node in clusterTree.
	// +optional
	NodeSelector NodeSelector `json:"nodeSelector,omitempty"`
}

type NodeSelector struct {
	// NodeName is Member cluster origin node Name
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// LabelSelector is a filter to select member cluster nodes to pretend a leaf node in clusterTree by labels.
	// It will work on second level schedule on pod create in member clusters.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type ClusterLinkStatus struct {
	// +optional
	PodCIDRs []string `json:"podCIDRs,omitempty"`
	// +optional
	ServiceCIDRs []string `json:"serviceCIDRs,omitempty"`
}

type ClusterTreeStatus struct {
	// LeafNodeItems represents list of the leaf node Items calculating in each member cluster.
	// +optional
	LeafNodeItems []LeafNodeItem `json:"leafNodeItems,omitempty"`
}

type LeafNodeItem struct {
	// LeafNodeName represents the leaf node name generate by controller.
	// suggest name format like cluster-shortLabel-number like member-az1-1
	// +required
	LeafNodeName string `json:"leafNodeName"`
}

type VxlanCIDRs struct {
	IP  string `json:"ip"`
	IP6 string `json:"ip6"`
}

type NICNodeNames struct {
	InterfaceName string   `json:"interfaceName"`
	NodeName      []string `json:"nodeName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Cluster `json:"items"`
}

func (c *Cluster) IsP2P() bool {
	if c.Spec.ClusterLinkOptions == nil {
		return false
	}
	return c.Spec.ClusterLinkOptions.NetworkType == NetworkTypeP2P
}

func (c *Cluster) IsGateway() bool {
	if c.Spec.ClusterLinkOptions == nil {
		return false
	}
	return c.Spec.ClusterLinkOptions.NetworkType == NetWorkTypeGateWay
}
