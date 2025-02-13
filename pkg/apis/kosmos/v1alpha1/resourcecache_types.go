/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceCache represents the configuration of the cache resource
type ResourceCache struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceCacheSpec   `json:"spec,omitempty"`
	Status ResourceCacheStatus `json:"status,omitempty"`
}

// ResourceCacheSpec defines the desired state of ResourceCache
type ResourceCacheSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ResourceSelectors specifies the resources type that should be cached by cache system.
	// +required
	ResourceCacheSelectors []ResourceCacheSelector `json:"resourceSelectors"`
}

// ResourceSelector specifies the resources type and its scope.
type ResourceCacheSelector struct {
	// APIVersion represents the API version of the target resources.
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// Kind represents the kind of the target resources.
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Namespace of the target resource.
	// Default is empty, which means all namespaces.
	// +kubebuilder:validation:Optional
	Namespace []string `json:"namespace,omitempty"`
}

// ResourceCacheStatus defines the observed state of ResourceCache
type ResourceCacheStatus struct {
	// Conditions contain the different condition statuses.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceCacheList contains a list of ResourceCache
type ResourceCacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceCache `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Proxying define a flag for resource proxying that do not have actual resources.
type Proxying struct {
	metav1.TypeMeta `json:",inline"`
}
