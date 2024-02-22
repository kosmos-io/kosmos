package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pc
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterPodConvertPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the PodConvertPolicy.
	// +required
	Spec ClusterPodConvertPolicySpec `json:"spec"`
}

type ClusterPodConvertPolicySpec struct {
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterPodConvertPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ClusterPodConvertPolicy `json:"items"`
}
