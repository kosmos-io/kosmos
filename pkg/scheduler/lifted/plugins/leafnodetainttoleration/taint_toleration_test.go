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

package leafnodetainttoleration

import (
	"context"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func nodeWithTaints(nodeName string, taints []v1.Taint) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: v1.NodeSpec{
			Taints: taints,
		},
	}
}

func podWithTolerations(podName string, tolerations []v1.Toleration) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: v1.PodSpec{
			Tolerations: tolerations,
		},
	}
}

func TestTaintTolerationFilter(t *testing.T) {
	tests := []struct {
		name       string
		pod        *v1.Pod
		node       *v1.Node
		wantStatus *framework.Status
	}{
		{
			name: "A pod having kosmos tolerations",
			pod:  podWithTolerations("pod1", []v1.Toleration{}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "kosmos.io/node",
					Value:  "true",
					Effect: v1.TaintEffectNoSchedule,
				},
			}),
		},
		{
			name: "A pod having no tolerations can't be scheduled onto a node with nonempty taints",
			pod:  podWithTolerations("pod1", []v1.Toleration{}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "dedicated",
					Value:  "user1",
					Effect: "NoSchedule",
				},
			}),
			wantStatus: framework.NewStatus(framework.UnschedulableAndUnresolvable,
				"node(s) had taint {dedicated: user1}, that the pod didn't tolerate"),
		},
		{
			name: "A pod which can be scheduled on a dedicated node assigned to user1 with effect NoSchedule",
			pod: podWithTolerations("pod1", []v1.Toleration{
				{
					Key:    "dedicated",
					Value:  "user1",
					Effect: "NoSchedule",
				},
			}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "dedicated",
					Value:  "user1",
					Effect: "NoSchedule",
				},
			}),
		},
		{
			name: "A pod which can't be scheduled on a dedicated node assigned to user2 with effect NoSchedule",
			pod: podWithTolerations("pod1", []v1.Toleration{
				{
					Key:      "dedicated",
					Operator: "Equal",
					Value:    "user2",
					Effect:   "NoSchedule",
				},
			}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "dedicated",
					Value:  "user1",
					Effect: "NoSchedule",
				},
			}),
			wantStatus: framework.NewStatus(framework.UnschedulableAndUnresolvable,
				"node(s) had taint {dedicated: user1}, that the pod didn't tolerate"),
		},
		{
			name: "A pod can be scheduled onto the node, with a toleration uses operator Exists that tolerates the taints on the node",
			pod: podWithTolerations("pod1", []v1.Toleration{
				{
					Key:      "foo",
					Operator: "Exists",
					Effect:   "NoSchedule",
				},
			}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: "NoSchedule",
				},
			}),
		},
		{
			name: "A pod has multiple tolerations, node has multiple taints, all the taints are tolerated, pod can be scheduled onto the node",
			pod: podWithTolerations("pod1", []v1.Toleration{
				{
					Key:      "dedicated",
					Operator: "Equal",
					Value:    "user2",
					Effect:   "NoSchedule",
				},
				{
					Key:      "foo",
					Operator: "Exists",
					Effect:   "NoSchedule",
				},
			}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "dedicated",
					Value:  "user2",
					Effect: "NoSchedule",
				},
				{
					Key:    "foo",
					Value:  "bar",
					Effect: "NoSchedule",
				},
			}),
		},
		{
			name: "A pod has a toleration that keys and values match the taint on the node, but (non-empty) effect doesn't match, " +
				"can't be scheduled onto the node",
			pod: podWithTolerations("pod1", []v1.Toleration{
				{
					Key:      "foo",
					Operator: "Equal",
					Value:    "bar",
					Effect:   "PreferNoSchedule",
				},
			}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: "NoSchedule",
				},
			}),
			wantStatus: framework.NewStatus(framework.UnschedulableAndUnresolvable,
				"node(s) had taint {foo: bar}, that the pod didn't tolerate"),
		},
		{
			name: "The pod has a toleration that keys and values match the taint on the node, the effect of toleration is empty, " +
				"and the effect of taint is NoSchedule. Pod can be scheduled onto the node",
			pod: podWithTolerations("pod1", []v1.Toleration{
				{
					Key:      "foo",
					Operator: "Equal",
					Value:    "bar",
				},
			}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: "NoSchedule",
				},
			}),
		},
		{
			name: "The pod has a toleration that key and value don't match the taint on the node, " +
				"but the effect of taint on node is PreferNoSchedule. Pod can be scheduled onto the node",
			pod: podWithTolerations("pod1", []v1.Toleration{
				{
					Key:      "dedicated",
					Operator: "Equal",
					Value:    "user2",
					Effect:   "NoSchedule",
				},
			}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "dedicated",
					Value:  "user1",
					Effect: "PreferNoSchedule",
				},
			}),
		},
		{
			name: "The pod has no toleration, " +
				"but the effect of taint on node is PreferNoSchedule. Pod can be scheduled onto the node",
			pod: podWithTolerations("pod1", []v1.Toleration{}),
			node: nodeWithTaints("nodeA", []v1.Taint{
				{
					Key:    "dedicated",
					Value:  "user1",
					Effect: "PreferNoSchedule",
				},
			}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodeInfo := framework.NewNodeInfo()
			nodeInfo.SetNode(test.node)
			p, _ := New(nil, nil)
			gotStatus := p.(framework.FilterPlugin).Filter(context.Background(), nil, test.pod, nodeInfo)
			if !reflect.DeepEqual(gotStatus, test.wantStatus) {
				t.Errorf("status does not match: %v, want: %v", gotStatus, test.wantStatus)
			}
		})
	}
}
