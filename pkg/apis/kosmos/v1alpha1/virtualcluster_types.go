package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Phase string

const (
	// Preparing means kubernetes control plane is preparing,and kubeconfig is not ready
	Preparing Phase = "Preparing"
	// Initialized means kubernetes control plane is ready,and kubeconfig is ready for use
	Initialized Phase = "Initialized"
	// Completed means everything is ready,kosmos is joined, and resource is promoted
	Completed Phase = "Completed"
	// AllNodeReady means all nodes have joined the virtual control plane and are in the running state
	AllNodeReady Phase = "AllNodeReady"
	// AllNodeDeleted means all nodes have been deleted
	AllNodeDeleted Phase = "AllNodeDeleted"
	// Deleting means virtualcluster is being deleted
	Deleting Phase = "Deleting"
	// Updating means that some changes are happening
	Updating Phase = "Updating"
	Pending  Phase = "Pending"
)

// +genclient
// +kubebuilder:resource:scope=Namespaced,shortName=vc
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="UPDATE-TIME",type=string,JSONPath=`.status.updateTime`

type VirtualCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the VirtualCluster.
	// +required
	Spec VirtualClusterSpec `json:"spec"`

	// Status describes the current status of a VirtualCluster.
	// +optional
	Status VirtualClusterStatus `json:"status,omitempty"`
}

type VirtualClusterSpec struct {
	// Kubeconfig is the kubeconfig of the virtual kubernetes's control plane
	// +optional
	Kubeconfig string `json:"kubeconfig,omitempty"`

	// ExternalIP is the external ip of the virtual kubernetes's control plane
	// +optional
	ExternalIP string `json:"externalIP,omitempty"`

	// PromotePolicies definites the policies for promote to the kubernetes's control plane
	// +required
	PromotePolicies []PromotePolicy `json:"promotePolicies,omitempty"`

	// PromoteResources definites the resources for promote to the kubernetes's control plane,
	// the resources can be nodes or just cpu,memory or gpu resources
	// +optional
	PromoteResources PromoteResources `json:"promoteResources,omitempty"`

	// datasource for plugin yaml
	// +optional
	PluginOptions []PluginOptions `json:"pluginOptions,omitempty"`
}

type PluginOptions struct {
	// +required
	Name string `json:"name"`
	// +required
	Value string `json:"value"`
}

type PromotePolicy struct {
	// LabelSelector is used to select nodes that are eligible for promotion to the kubernetes's control plane.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// NodeCount is the number of nodes to promote to the kubernetes's control plane
	// +required
	NodeCount int32 `json:"nodeCount"`
}

type PromoteResources struct {
	// NodeInfos is the info of nodes to promote to the kubernetes's control plane
	// +optional
	NodeInfos []NodeInfo `json:"nodeInfos,omitempty"`

	// Resources is the resources to promote to the kubernetes's control plane
	// +optional
	Resources corev1.ResourceList `json:"resources,omitempty"`
}

type NodeInfo struct {
	//NodeName defines node name
	//+optional
	NodeName string `json:"nodeName,omitempty"`
}

type VirtualClusterStatus struct {
	// Phase is the phase of kosmos-operator handling the VirtualCluster
	// +optional
	Phase Phase `json:"phase,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`
	// +optional
	UpdateTime *metav1.Time `json:"updateTime,omitempty" protobuf:"bytes,7,opt,name=updateTime"`
	// +optional
	Port int32 `json:"port,omitempty"`
	// +optional
	PortMap map[string]int32 `json:"portMap,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type VirtualClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []VirtualCluster `json:"items"`
}

const UseTenantCoreDns = "kosmos-io/use-tenant-core-dns"

func (v *VirtualCluster) UseTenantCoreDns() bool {
	// default: false
	ans := v.GetAnnotations()
	if ans == nil {
		return false
	}
	if value, ok := ans[UseTenantCoreDns]; ok {
		return value == "true"
	}
	return false
}
