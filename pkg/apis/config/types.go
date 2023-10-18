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

// MaxCustomPriorityScore is the max score UtilizationShapePoint expects.
const MaxCustomPriorityScore int64 = 10

// UtilizationShapePoint represents a single point of a priority function shape.
type UtilizationShapePoint struct {
	// Utilization (x axis). Valid values are 0 to 100. Fully utilized node maps to 100.
	Utilization int32
	// Score assigned to a given utilization (y axis). Valid values are 0 to 10.
	Score int32
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KnodeVolumeBindingArgs holds arguments used to configure the KnodeVolumeBindingArgs plugin.
type KnodeVolumeBindingArgs struct {
	metav1.TypeMeta `json:",inline"`

	// BindTimeoutSeconds is the timeout in seconds in volume binding operation.
	// Value must be non-negative integer. The value zero indicates no waiting.
	// If this value is nil, the default value will be used.
	BindTimeoutSeconds int64 `json:"bindTimeoutSeconds,omitempty"`

	// Shape specifies the points defining the score function shape, which is
	// used to score nodes based on the utilization of statically provisioned
	// PVs. The utilization is calculated by dividing the total requested
	// storage of the pod by the total capacity of feasible PVs on each node.
	// Each point contains utilization (ranges from 0 to 100) and its
	// associated score (ranges from 0 to 10). You can turn the priority by
	// specifying different scores for different utilization numbers.
	// The default shape points are:
	// 1) 0 for 0 utilization
	// 2) 10 for 100 utilization
	// All points must be sorted in increasing order by utilization.
	// +featureGate=VolumeCapacityPriority
	// +optional
	Shape []UtilizationShapePoint
}
