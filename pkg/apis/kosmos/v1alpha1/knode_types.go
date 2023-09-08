package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type AdapterType string

const (
	K8sAdapter AdapterType = "k8s"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Knode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec KnodeSpec `json:"spec,omitempty"`

	// +optional
	Status KnodeStatus `json:"status,omitempty"`
}

type KnodeSpec struct {
	// +optional
	Kubeconfig []byte `json:"kubeconfig,omitempty"`

	// +required
	NodeName string `json:"nodeName,omitempty"`

	// +kubebuilder:default=k8s
	// +optional
	Type AdapterType `json:"type,omitempty"`

	// +optional
	DisableTaint bool `json:"disableTaint,omitempty"`
}

type KnodeStatus struct {
	// +optional
	APIServer string `json:"apiserver,omitempty"`

	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KnodeList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Knode `json:"items"`
}
