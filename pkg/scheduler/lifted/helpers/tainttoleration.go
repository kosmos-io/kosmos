/*
Copyright 2017 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
//https://github.com/kubernetes/component-helpers/blob/master/scheduling/corev1/helpers.go

package helpers

import v1 "k8s.io/api/core/v1"

var KnodeTaint = &v1.Taint{
	Key:    "knode.kosmos.io",
	Value:  "true",
	Effect: v1.TaintEffectNoSchedule,
}

// TolerationsTolerateTaint checks if taint is tolerated by any of the tolerations.
func TolerationsTolerateTaint(tolerations []v1.Toleration, taint *v1.Taint) bool {
	if taint.MatchTaint(KnodeTaint) {
		return true
	}
	for i := range tolerations {
		if tolerations[i].ToleratesTaint(taint) {
			return true
		}
	}
	return false
}

type taintsFilterFunc func(*v1.Taint) bool

// FindMatchingUntoleratedTaint checks if the given tolerations tolerates
// all the filtered taints, and returns the first taint without a toleration
// Returns true if there is an untolerated taint
// Returns false if all taints are tolerated
func FindMatchingUntoleratedTaint(taints []v1.Taint, tolerations []v1.Toleration, inclusionFilter taintsFilterFunc) (v1.Taint, bool) {
	filteredTaints := getFilteredTaints(taints, inclusionFilter)
	for _, taint := range filteredTaints {
		localTaint := taint
		if !TolerationsTolerateTaint(tolerations, &localTaint) {
			return taint, true
		}
	}
	return v1.Taint{}, false
}

// getFilteredTaints returns a list of taints satisfying the filter predicate
func getFilteredTaints(taints []v1.Taint, inclusionFilter taintsFilterFunc) []v1.Taint {
	if inclusionFilter == nil {
		return taints
	}
	filteredTaints := []v1.Taint{}
	for _, taint := range taints {
		localTaint := taint
		if !inclusionFilter(&localTaint) {
			continue
		}
		filteredTaints = append(filteredTaints, localTaint)
	}
	return filteredTaints
}
