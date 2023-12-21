package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// LeafNodeDistributionArgs holds arguments used to configure the LeafNodeDistribution plugin
type LeafNodeDistributionArgs struct {
	metav1.TypeMeta `json:",inline"`

	// KubeConfigPath is the path of kubeconfig.
	KubeConfigPath string `json:"kubeConfigPath,omitempty"`
}
