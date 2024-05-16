package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vp
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type VirtualClusterPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// VirtualClusterPluginSpec is the specification for a VirtualClusterPlugin resource
	// +required
	Spec VirtualClusterPluginSpec `json:"spec"`
}

type VirtualClusterPluginSpec struct {
	// +optional
	PluginSources PluginSources `json:"pluginSources,omitempty"`

	// +optional
	SuccessStateCommand string `json:"successStateCommand,omitempty"`
}

type PluginSources struct {
	// +optional
	Chart Chart `json:"chart,omitempty"`
	// +optional
	Yaml Yaml `json:"yaml,omitempty"`
}

type Chart struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Repo string `json:"repo,omitempty"`
	// +optional
	Storage Storage `json:"storage,omitempty"`
	// +optional
	Version string `json:"version,omitempty"`
	// +optional
	ValuesFile Storage `json:"valuesFile,omitempty"`
	// +optional
	Values []string `json:"values,omitempty"`
	// +optional
	Wait bool `json:"wait,omitempty"`
}

type Yaml struct {
	// +required
	Path Storage `json:"path"`
}

type Storage struct {
	// +optional
	HostPath HostPath `json:"hostPath,omitempty"`

	// +optional
	PVPath string `json:"pvPath,omitempty"`

	// +optional
	URI string `json:"uri,omitempty"`
}

type HostPath struct {
	// +optional
	Path string `json:"path,omitempty"`

	// +optional
	NodeName string `json:"nodeName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type VirtualClusterPluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []VirtualClusterPlugin `json:"items"`
}
