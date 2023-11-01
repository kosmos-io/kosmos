package v1alpha1

import (
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
	ClusterLinkOptions ClusterLinkOptions `json:"clusterLinkOptions,omitempty"`

	// +optional
	ClusterTreeOptions ClusterTreeOptions `json:"clusterTreeOptions,omitempty"`
}

type ClusterStatus struct {
	ClusterLinkStatus ClusterLinkStatus `json:"clusterLinkStatus,omitempty"`
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
}

type ClusterTreeOptions struct {
	// +kubebuilder:default=true
	// +optional
	Enable bool `json:"enable"`
}

type ClusterLinkStatus struct {
	// +optional
	PodCIDRs []string `json:"podCIDRs,omitempty"`
	// +optional
	ServiceCIDRs []string `json:"serviceCIDRs,omitempty"`
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
	return c.Spec.ClusterLinkOptions.NetworkType == NetworkTypeP2P
}

func (c *Cluster) IsGateway() bool {
	return c.Spec.ClusterLinkOptions.NetworkType == NetWorkTypeGateWay
}
