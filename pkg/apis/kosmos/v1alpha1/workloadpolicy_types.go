package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:resource:shortName=wlp,categories={kosmos-io}
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp",description="CreationTimestamp is a timestamp representing the server time when this object was created. It is not guaranteed to be set in happens-before order across separate operations. Clients may not set this value. It is represented in RFC3339 form and is in UTC."

type WorkloadPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec represents the desired behavior of WorkloadPolicyPolicy.
	// +required
	Spec WorkloadPolicySpec `json:"spec,omitempty"`
}

// WorkloadPolicySpec represents the desired behavior of WorkloadPolicyPolicy.
type WorkloadPolicySpec struct {
	// TopologyKey is used when match node topologyKey
	// +required
	TopologyKey string `json:"topologyKey"`

	// LabelSelector is used to filter matching pods.
	// +required
	LabelSelector *metav1.LabelSelector `json:"labelSelector"`

	// AllocationPolicy describes the allocation policy when scheduling pods.
	// +required
	AllocationPolicy []AllocationPolicy `json:"allocationPolicy"`

	// AllocationType sets the scheduling constraint to schedule pods for a WorkloadPolicy.
	// Valid options are Required or Preferred.
	// Required means that the pods will get scheduled only on nodes that has a topologyKey=topologyValue label.
	// Preferred means that the pods will prefer nodes that has a topologyKey=topologyValue label.
	// +kubebuilder:default="Preferred"
	// +kubebuilder:validation:Enum=Preferred;Required
	// +optional
	AllocationType string `json:"allocationType"`

	// AllocationMethod sets the scheduling method to schedule pods for a WorkloadPolicy.
	// Valid options are Fill or Balance.
	// Fill, pods with the same label are scheduled in fill mode between nodes in the same topology.
	// Balance, pods with the same label are scheduled in balance mode between nodes in the same topology.
	// +kubebuilder:default="Balance"
	// +kubebuilder:validation:Enum=Fill;Balance
	// +optional
	AllocationMethod string `json:"allocationMethod"`
}

// AllocationPolicy set the topologyValue required replicas
type AllocationPolicy struct {
	// Name is the topology value
	// +required
	Name string `json:"name"`

	// Replicas is the desired the replicas for the topology value
	// +required
	Replicas int32 `json:"replicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WorkloadPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []WorkloadPolicy `json:"items"`
}
