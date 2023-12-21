package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp",description="CreationTimestamp is a timestamp representing the server time when this object was created. It is not guaranteed to be set in happens-before order across separate operations. Clients may not set this value. It is represented in RFC3339 form and is in UTC."

type DistributionPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DistributionSpec represents the desired behavior of DistributionPolicy.
	// +required
	DistributionSpec DistributionSpec `json:"spec"`
}

// DistributionSpec represents the desired behavior of DistributionPolicy.
type DistributionSpec struct {
	// ResourceSelectors used to select resources and is required.
	// +required
	// +kubebuilder:validation:MinItems=1
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// PolicyTerms represents the rule for select nodes to distribute resources.
	// +required
	// +kubebuilder:validation:MinItems=1
	PolicyTerms []PolicyTerm `json:"policyTerms"`
}

// ResourceSelector the resources will be selected.
type ResourceSelector struct {
	// Name of the Policy.
	// +required
	PolicyName string `json:"policyName"`

	// Namespace of the target resource.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`

	// NamespacePrefix the prefix of the target resource namespace
	// +optional
	NamespacePrefix string `json:"namespacePrefix,omitempty"`

	// Filter resource by labelSelector
	// If target resource name is not empty, labelSelector will be ignored.
	// +optional
	NamespaceLabelSelector *metav1.LabelSelector `json:"namespaceLabelSelector,omitempty"`

	// Name of the target resource.
	// Default is empty, which means selecting all resources.
	// +optional
	Name string `json:"name,omitempty"`

	// NamePrefix the prefix of the target resource name
	// +optional
	NamePrefix string `json:"namePrefix,omitempty"`

	// Filter resource by labelSelector
	// If target resource name is not empty, labelSelector will be ignored.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type NodeType string

const (
	// HOSTNODE represents only the host nodes
	HOSTNODE NodeType = "host"

	// LEAFNODE represents only the leaf nodes
	LEAFNODE NodeType = "leaf"

	//MIXNODE represents the host nodes and child nodes
	MIXNODE NodeType = "mix"
)

type PolicyTerm struct {
	// +required
	Name string `json:"name"`

	// NodeType declares the type for scheduling node.
	// Valid options are "host", "leaf", "mix".
	//
	// +kubebuilder:default="mix"
	// +kubebuilder:validation:Enum=host;leaf;mix
	// +optional
	NodeType NodeType `json:"nodeType,omitempty"`

	// AdvancedTerm represents scheduling restrictions to a certain set of nodes.
	// +optional
	AdvancedTerm AdvancedTerm `json:"advancedTerm,omitempty"`
}

// AdvancedTerm represents scheduling restrictions to a certain set of nodes.
type AdvancedTerm struct {
	// NodeName is a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	// +optional
	NodeName string `json:"nodeName,omitempty" protobuf:"bytes,10,opt,name=nodeName"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`

	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty" protobuf:"bytes,18,opt,name=affinity"`

	// If specified, the pod's tolerations.
	// +optional
	Tolerations []*corev1.Toleration `json:"tolerations,omitempty" protobuf:"bytes,22,opt,name=tolerations"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DistributionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DistributionPolicy `json:"items"`
}
