package v1alpha1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="ROLES",type=string,JSONPath=`.spec.roles`
// +kubebuilder:printcolumn:name="INTERFACE",type=string,JSONPath=`.spec.interfaceName`
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.spec.ip`

type ClusterNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterNodeSpec `json:"spec"`

	// +optional
	Status ClusterNodeStatus `json:"status"`
}

type ClusterNodeSpec struct {
	ClusterName string `json:"clusterName,omitempty"`
	NodeName    string `json:"nodeName,omitempty"`
	// +optional
	PodCIDRs []string `json:"podCIDRs,omitempty"`
	// +optional
	IP string `json:"ip,omitempty"`
	// +optional
	IP6 string `json:"ip6,omitempty"`
	// +optional
	Roles []Role `json:"roles,omitempty"`
	// +optional
	InterfaceName string `json:"interfaceName"`
}

type ClusterNodeStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ClusterNode `json:"items"`
}

func hasRole(roles []Role, r Role) bool {
	s := string(r)
	for _, role := range roles {
		str := string(role)
		if strings.EqualFold(s, str) {
			return true
		}
	}
	return false
}

func (c *ClusterNode) IsGateway() bool {
	return hasRole(c.Spec.Roles, RoleGateway)
}
