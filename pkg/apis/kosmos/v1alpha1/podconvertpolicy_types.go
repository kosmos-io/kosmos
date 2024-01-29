package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pc
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodConvertPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the PodConvertPolicy.
	// +required
	Spec PodConvertPolicySpec `json:"spec"`
}

type PodConvertPolicySpec struct {
	// A label query over a set of resources.
	// If name is not empty, labelSelector will be ignored.
	// +required
	LabelSelector metav1.LabelSelector `json:"labelSelector"`

	// A label query over a set of resources.
	// If name is not empty, LeafNodeSelector will be ignored.
	// +option
	LeafNodeSelector *metav1.LabelSelector `json:"leafNodeSelector,omitempty"`

	// Converters are some converter for convert pod when pod synced from root cluster to leaf cluster
	// pod will use these converters to scheduled in leaf cluster
	// +optional
	Converters *Converters `json:"converters,omitempty"`
}

// Converters are some converter for pod to scheduled in leaf cluster
type Converters struct {
	// +optional
	SchedulerNameConverter *SchedulerNameConverter `json:"schedulerNameConverter,omitempty"`
	// +optional
	NodeNameConverter *NodeNameConverter `json:"nodeNameConverter,omitempty"`
	// +optional
	NodeSelectorConverter *NodeSelectorConverter `json:"nodeSelectorConverter,omitempty"`
	// +optional
	TolerationConverter *TolerationConverter `json:"tolerationConverter,omitempty"`
	// +optional
	AffinityConverter *AffinityConverter `json:"affinityConverter,omitempty"`
	// +optional
	TopologySpreadConstraintsConverter *TopologySpreadConstraintsConverter `json:"topologySpreadConstraintsConverter,omitempty"`
}

// ConvertType if the operation type when convert pod from root cluster to leaf cluster.
type ConvertType string

// These are valid conversion types.
const (
	Add     ConvertType = "add"
	Remove  ConvertType = "remove"
	Replace ConvertType = "replace"
)

// SchedulerNameConverter used to modify the pod's nodeName when pod synced to leaf cluster
type SchedulerNameConverter struct {
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	ConvertType ConvertType `json:"convertType"`

	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`
}

// NodeNameConverter used to modify the pod's nodeName when pod synced to leaf cluster
type NodeNameConverter struct {
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	ConvertType ConvertType `json:"convertType"`

	// +optional
	NodeName string `json:"nodeName,omitempty"`
}

// NodeSelectorConverter used to modify the pod's NodeSelector when pod synced to leaf cluster
type NodeSelectorConverter struct {
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	ConvertType ConvertType `json:"convertType"`

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// TolerationConverter used to modify the pod's Tolerations when pod synced to leaf cluster
type TolerationConverter struct {
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	ConvertType ConvertType `json:"convertType"`

	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// AffinityConverter used to modify the pod's Affinity when pod synced to leaf cluster
type AffinityConverter struct {
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	ConvertType ConvertType `json:"convertType"`

	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// TopologySpreadConstraintsConverter used to modify the pod's TopologySpreadConstraints when pod synced to leaf cluster
type TopologySpreadConstraintsConverter struct {
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	ConvertType ConvertType `json:"convertType"`

	// TopologySpreadConstraints describes how a group of pods ought to spread across topology
	// domains. Scheduler will schedule pods in a way which abides by the constraints.
	// All topologySpreadConstraints are ANDed.
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodConvertPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PodConvertPolicy `json:"items"`
}
