package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlobalNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the GlobalNodeSpec.
	// +required
	Spec GlobalNodeSpec `json:"spec"`

	// +optional
	Status GlobalNodeStatus `json:"status,omitempty"`
}

type GlobalNodeSpec struct {
	// +optional
	NodeIP string `json:"nodeIP,omitempty"`

	// +kubebuilder:default=free
	// +optional
	State NodeState `json:"state,omitempty"`

	// +optional
	Labels labels.Set `json:"labels,omitempty"`
}

type NodeState string

const (
	NodeInUse     NodeState = "occupied"
	NodeFreeState NodeState = "free"
	NodeReserved  NodeState = "reserved"
)

type GlobalNodeStatus struct {
	// +optional
	VirtualCluster string `json:"virtualCluster,omitempty"`

	// Conditions is an array of current observed node conditions.
	// More info: https://kubernetes.io/docs/concepts/nodes/node/#condition
	// +optional
	Conditions []corev1.NodeCondition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlobalNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []GlobalNode `json:"items"`
}
