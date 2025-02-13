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

package helpers

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

const (
	// WorkloadPolicyLabelKey defines the label key for workload-policy.
	WorkloadPolicyLabelKey = "workload-policy/kosmos.io"

	// ErrReasonConstraintsNotMatch indicates workload-policy filter constraints not met.
	ErrReasonConstraintsNotMatch = "node(s) didn't match pod workload-policy constraints"

	// ErrReasonNodeLabelNotMatch indicates a missing required node label.
	ErrReasonNodeLabelNotMatch = ErrReasonConstraintsNotMatch + " (missing required label)"

	ErrReasonConstraintsNotMatchValue = " (not match value)"

	// ErrReasonReachReplicas indicates the scheduled pods reach the replicas configured in the workload-policy.
	ErrReasonReachReplicas = "node(s) occupied that scheduled pods reach the replicas configured in the workload-policy"

	// AllocationMethodFill pods with the same label are scheduled in fill mode between nodes in the same topology.
	AllocationMethodFill = "Fill"

	// AllocationMethodBalance pods with the same label are scheduled in balance mode between nodes in the same topology.
	AllocationMethodBalance = "Balance"

	// AllocationTypePreferred means that the pods will prefer nodes that has a topologyKey=topologyValue label.
	AllocationTypePreferred = "Preferred"

	// AllocationTypeRequired means that the pods will get scheduled only on nodes that has a topologyKey=topologyValue label.
	AllocationTypeRequired = "Required"
)

// WorkloadPolicyConstraint defines constraints for workload policies.
type WorkloadPolicyConstraint struct {
	TopologyKey      string
	AllocationPolicy map[string]int32
	Selector         labels.Selector
}

// TopologyPair represents a topology key-value pair.
type TopologyPair struct {
	Key   string
	Value string
}

// ConvertPolicy converts a slice of AllocationPolicy to a map of policy names to replica counts.
func ConvertPolicy(policies []v1alpha1.AllocationPolicy) map[string]int32 {
	result := make(map[string]int32, len(policies))
	for _, policy := range policies {
		result[policy.Name] = policy.Replicas
	}
	return result
}

// HasWorkloadPolicyLabel checks if a pod has the workload policy label.
func HasWorkloadPolicyLabel(pod *v1.Pod) bool {
	_, exists := pod.Labels[WorkloadPolicyLabelKey]
	return exists
}

func CountPodsMatchSelector(podInfos []*framework.PodInfo, selector labels.Selector, ns string) int {
	count := 0
	for _, podInfo := range podInfos {
		if podInfo == nil {
			continue
		}
		pod := podInfo.Pod

		// Skip terminating pods or those in a different namespace. (see #87621).
		if pod.DeletionTimestamp != nil || pod.Namespace != ns {
			continue
		}

		// Match pod labels with the selector.
		podLabels := labels.Set(pod.Labels)
		if selector.Matches(podLabels) {
			count++
		}
	}

	return count
}

func CheckTopologyValueReached(tpKey, tpValue string, allocationPolicy map[string]int32,
	tpPairToMatchNum map[TopologyPair]*int32) (bool, error) {
	pair := TopologyPair{
		Key:   tpKey,
		Value: tpValue,
	}

	desired, ok := allocationPolicy[tpValue]
	if !ok {
		return true, fmt.Errorf(ErrReasonConstraintsNotMatch + ErrReasonConstraintsNotMatchValue)
	}

	count, ok := tpPairToMatchNum[pair]
	if !ok || count == nil {
		return false, nil
	}

	if *count < desired {
		return false, nil
	}

	return true, nil
}
