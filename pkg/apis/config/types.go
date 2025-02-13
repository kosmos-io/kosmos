/*
Copyright 2020 The Kubernetes Authors.

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

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LeafNodeVolumeBindingArgs holds arguments used to configure the LeafNodeVolumeBinding plugin
type LeafNodeVolumeBindingArgs struct {
	metav1.TypeMeta `json:",inline"`

	// BindTimeoutSeconds is the timeout in seconds in volume binding operation.
	// Value must be non-negative integer. The value zero indicates no waiting.
	// If this value is nil, the default value (600) will be used.
	BindTimeoutSeconds int64 `json:"bindTimeoutSeconds,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LeafNodeDistributionArgs holds arguments used to configure the LeafNodeDistribution plugin
type LeafNodeDistributionArgs struct {
	metav1.TypeMeta `json:",inline"`

	// KubeConfigPath is the path of kubeconfig.
	KubeConfigPath string `json:"kubeConfigPath,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LeafNodeWorkloadArgs defines the scheduling parameters for WorkloadPolicy plugin.
type LeafNodeWorkloadArgs struct {
	metav1.TypeMeta `json:",inline"`

	// KubeConfigPath is the path of kubeconfig.
	KubeConfigPath string `json:"kubeConfigPath,omitempty"`
}
