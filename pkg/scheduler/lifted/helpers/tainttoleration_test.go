/*
Copyright 2019 The Kubernetes Authors.

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
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestHasLeafNodeTaint(t *testing.T) {
	nodeWithTaint := &v1.Node{
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{
				{
					Key:    "kosmos.io/node",
					Value:  "true",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
		},
	}

	nodeWithoutTaint := &v1.Node{
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{},
		},
	}

	if !HasLeafNodeTaint(nodeWithTaint) {
		t.Errorf("Expected node to have LeafNodeTaint")
	}

	if HasLeafNodeTaint(nodeWithoutTaint) {
		t.Errorf("Expected node to not have LeafNodeTaint")
	}
}

func TestTolerationsTolerateTaint(t *testing.T) {
	taint := &v1.Taint{
		Key:    "kosmos.io/node",
		Value:  "true",
		Effect: v1.TaintEffectNoSchedule,
	}

	toleration := v1.Toleration{
		Key:      "kosmos.io/node",
		Operator: v1.TolerationOpEqual,
		Value:    "true",
		Effect:   v1.TaintEffectNoSchedule,
	}

	tolerations := []v1.Toleration{toleration}

	if !TolerationsTolerateTaint(tolerations, taint) {
		t.Errorf("Expected tolerations to tolerate the taint")
	}

	untoleratedTaint := &v1.Taint{
		Key:    "kosmos.io/node-test",
		Value:  "false",
		Effect: v1.TaintEffectNoSchedule,
	}

	if TolerationsTolerateTaint(tolerations, untoleratedTaint) {
		t.Errorf("Expected tolerations to not tolerate the untolerated taint")
	}
}

func TestFindMatchingUntoleratedTaint(t *testing.T) {
	taints := []v1.Taint{
		{
			Key:    "kosmos.io/node",
			Value:  "true",
			Effect: v1.TaintEffectNoSchedule,
		},
		{
			Key:    "example.io/another-taint",
			Value:  "true",
			Effect: v1.TaintEffectNoSchedule,
		},
	}

	tolerations := []v1.Toleration{
		{
			Key:      "example.io/another-taint",
			Operator: v1.TolerationOpEqual,
			Value:    "true",
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	taint, found := FindMatchingUntoleratedTaint(taints, tolerations, nil)
	if found {
		t.Errorf("Expected to find an untolerated taint")
	} else if taint.Key != "" && taint.Key != "kosmos.io/node" {
		t.Errorf("Expected untolerated taint to be 'kosmos.io/node', got '%s'", taint.Key)
	}
}

func TestGetFilteredTaints(t *testing.T) {
	taints := []v1.Taint{
		{
			Key:    "kosmos.io/node",
			Value:  "true",
			Effect: v1.TaintEffectNoSchedule,
		},
		{
			Key:    "example.io/another-taint",
			Value:  "true",
			Effect: v1.TaintEffectNoSchedule,
		},
	}

	filterFunc := func(t *v1.Taint) bool {
		return t.Key == "kosmos.io/node"
	}

	filtered := getFilteredTaints(taints, filterFunc)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered taint, got %d", len(filtered))
	} else if filtered[0].Key != "kosmos.io/node" {
		t.Errorf("Expected filtered taint to be 'kosmos.io/node', got '%s'", filtered[0].Key)
	}
}
