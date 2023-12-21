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

// This code is lifted from the Kubernetes codebase and make some slight modifications in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/scheduler/framework/plugins/tainttoleration/taint_toleration.go

package leafnodetainttoleration

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/helpers"
)

const Name = "LeafNodeTaintToleration"

type TaintToleration struct {
	frameworkHandler framework.Handle
}

// EventsToRegister returns the possible events that may make a Pod
// failed by this plugin schedulable.
func (t *TaintToleration) EventsToRegister() []framework.ClusterEvent {
	return []framework.ClusterEvent{
		{Resource: framework.Node, ActionType: framework.Add | framework.Update},
	}
}

func (t *TaintToleration) Name() string {
	return Name
}

func (t *TaintToleration) Filter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	if nodeInfo == nil || nodeInfo.Node() == nil {
		return framework.AsStatus(fmt.Errorf("invalid nodeInfo"))
	}

	filterPredicate := func(t *corev1.Taint) bool {
		// PodToleratesNodeTaints is only interested in NoSchedule and NoExecute taints.
		return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
	}

	taint, isUntolerated := helpers.FindMatchingUntoleratedTaint(nodeInfo.Node().Spec.Taints, pod.Spec.Tolerations, filterPredicate)
	if !isUntolerated {
		return nil
	}

	errReason := fmt.Sprintf("node(s) had taint {%s: %s}, that the pod didn't tolerate",
		taint.Key, taint.Value)
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, errReason)
}

var _ framework.FilterPlugin = &TaintToleration{}
var _ framework.EnqueueExtensions = &TaintToleration{}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &TaintToleration{frameworkHandler: h}, nil
}
