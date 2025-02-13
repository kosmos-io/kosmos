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

package leafnodeworkloadpolicy

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/helper"
	frameworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"

	"github.com/kosmos.io/kosmos/pkg/apis/config"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	schedinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	wpv1alpha1 "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/helpers"
)

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "LeafNodeWorkloadPolicy"

	// preFilterStateKey is the key in CycleState to NodeResourcesFit pre-computed data.
	preFilterStateKey = "PreFilter" + Name
)

// Prefilter build cache for scheduling cycle
// Filter Checks node filter.
// Score the nodes according to topology
// Reserve reserves the node/pod to cache, if failed run unreserve to clean up cache
var _ framework.PreFilterPlugin = &WorkloadPolicy{}
var _ framework.FilterPlugin = &WorkloadPolicy{}
var _ framework.ScorePlugin = &WorkloadPolicy{}
var _ framework.ReservePlugin = &WorkloadPolicy{}

// WorkloadPolicy is a leaf node scheduler plugin.
type WorkloadPolicy struct {
	handle       framework.Handle
	podLister    corelisters.PodLister
	policyLister wpv1alpha1.WorkloadPolicyLister
}

// Name returns name of the plugin.
func (wp *WorkloadPolicy) Name() string {
	return Name
}

// initWorkloadPolicyLister initializes the WorkloadPolicyLister using the provided kubeconfig path.
func initWorkloadPolicyLister(kubeConfigPath *string) (*wpv1alpha1.WorkloadPolicyLister, error) {
	// Build the REST config from the provided kubeconfig path
	restConfig, err := clientcmd.BuildConfigFromFlags("", *kubeConfigPath)
	if err != nil {
		klog.ErrorS(err, "Failed to create kubeconfig", "kubeConfigPath", *kubeConfigPath)
		return nil, fmt.Errorf("failed to build kubeconfig from path %v: %v", *kubeConfigPath, err)
	}

	// Create the client for interacting with the API server
	client, err := versioned.NewForConfig(restConfig)
	if err != nil {
		klog.ErrorS(err, "Failed to create client for LeafNodeWorkloadPolicy", "kubeConfig", restConfig)
		return nil, fmt.Errorf("failed to create client for LeafNodeWorkloadPolicy: %v", err)
	}

	// Initialize the shared informer factory for WorkloadPolicies
	schedSharedInformerFactory := schedinformer.NewSharedInformerFactory(client, 0)
	wpInformer := schedSharedInformerFactory.Kosmos().V1alpha1().WorkloadPolicies().Informer()
	wpLister := schedSharedInformerFactory.Kosmos().V1alpha1().WorkloadPolicies().Lister()

	klog.V(5).InfoS("Starting WorkloadPolicyInformer")

	// Start the informer factory and sync the cache
	schedSharedInformerFactory.Start(nil)

	// Wait for the cache to sync, and return an error if the sync times out
	if !cache.WaitForCacheSync(nil, wpInformer.HasSynced) {
		return nil, fmt.Errorf("timed out waiting for caches to sync %v", Name)
	}

	klog.InfoS("Successfully synchronized WorkloadPolicy informer")

	// Return the lister once the cache is synchronized
	return &wpLister, nil
}

// New initializes a new plugin and returns it.
func New(plArgs k8sruntime.Object, handle framework.Handle) (framework.Plugin, error) {
	klog.V(5).InfoS("Creating new LeafNodeWorkloadPolicy plugin")

	var args config.LeafNodeWorkloadArgs
	err := frameworkruntime.DecodeInto(plArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("want args to be of type WorkloadPolicyArgs, got %T", plArgs)
	}

	// init WorkloadPolicyLister
	wpLister, err := initWorkloadPolicyLister(&args.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init WorkloadPolicyLister: %v", err)
	}

	wp := &WorkloadPolicy{
		handle:       handle,
		podLister:    handle.SharedInformerFactory().Core().V1().Pods().Lister(),
		policyLister: *wpLister,
	}

	klog.Infof("LeafNodeWorkloadPolicy plugin started successfully")
	return wp, nil
}

// PreFilterState stores information computed at PreFilter and used at PostFilter or Reserve.
type PreFilterState struct {
	Constraint helpers.WorkloadPolicyConstraint

	// TpPairToMatchNum maps TopologyPair to the number of matching pods.
	TpPairToMatchNum map[helpers.TopologyPair]*int32

	// AllocationMethod indicates how allocation is performed (Balance or Fill).
	AllocationMethod string

	// AllocationType indicates whether allocation is Preferred or Required.
	AllocationType string
}

// Clone is a function to make a copy of StateData. For performance reasons,
// clone should make shallow copies for members (e.g., slices or maps) that are not
// impacted by PreFilter's optional AddPod/RemovePod methods.
func (pfs *PreFilterState) Clone() framework.StateData {
	if pfs == nil {
		klog.Infof("PreFilterState is nil when cloning")
		return nil
	}

	pfsCopy := PreFilterState{
		Constraint:       pfs.Constraint,
		TpPairToMatchNum: make(map[helpers.TopologyPair]*int32, len(pfs.TpPairToMatchNum)),
		AllocationMethod: pfs.AllocationMethod,
		AllocationType:   pfs.AllocationType,
	}

	for pair, num := range pfs.TpPairToMatchNum {
		copyPair := helpers.TopologyPair{
			Key:   pair.Key,
			Value: pair.Value,
		}

		copyCount := *num
		pfsCopy.TpPairToMatchNum[copyPair] = &copyCount
	}

	return &pfsCopy
}

func (wp *WorkloadPolicy) calculatePreFilterState(pod *v1.Pod) (*PreFilterState, error) {
	wpName, podNs, podName := pod.Labels[helpers.WorkloadPolicyLabelKey], pod.Namespace, pod.Name

	tsp, err := wp.policyLister.WorkloadPolicies(podNs).Get(wpName)
	if err != nil {
		return nil, fmt.Errorf("Fail to obtain pod's(%s/%s) workload-policy(%s): %v ", podNs, podName, wpName, err)
	}

	allNodes, err := wp.handle.SnapshotSharedLister().NodeInfos().List()
	if err != nil {
		return nil, fmt.Errorf("Fail to list NodeInfos: %v ", err)
	}

	tspLabelSelector := tsp.Spec.LabelSelector
	if tspLabelSelector == nil {
		return nil, fmt.Errorf("workload-policy %v has no label selector", wpName)
	}
	labels := labels.Set(tspLabelSelector.MatchLabels)
	selector := labels.AsSelector()

	tpKey := tsp.Spec.TopologyKey
	constraint := helpers.WorkloadPolicyConstraint{
		TopologyKey:      tpKey,
		AllocationPolicy: helpers.ConvertPolicy(tsp.Spec.AllocationPolicy),
		Selector:         selector,
	}

	pfs := PreFilterState{
		Constraint:       constraint,
		TpPairToMatchNum: make(map[helpers.TopologyPair]*int32),
		AllocationMethod: tsp.Spec.AllocationMethod,
		AllocationType:   tsp.Spec.AllocationType,
	}

	for _, n := range allNodes {
		node := n.Node()
		if node == nil || !helper.PodMatchesNodeSelectorAndAffinityTerms(pod, node) {
			klog.Error("node not found")
			continue
		}

		// check node labels
		tpVal, ok := node.Labels[tpKey]
		if !ok {
			continue
		}

		pair := helpers.TopologyPair{
			Key:   tpKey,
			Value: tpVal,
		}
		pfs.TpPairToMatchNum[pair] = new(int32)
	}

	countPodsForNode := func(index int) {
		nodeInfo := allNodes[index]
		node := nodeInfo.Node()

		pair := helpers.TopologyPair{
			Key:   tpKey,
			Value: node.Labels[tpKey],
		}
		tpCount, ok := pfs.TpPairToMatchNum[pair]
		if !ok {
			return
		}

		count := helpers.CountPodsMatchSelector(nodeInfo.Pods, pfs.Constraint.Selector, podNs)
		atomic.AddInt32(tpCount, int32(count))
	}

	// the number of logical CPUs usable by the current process
	numCPU := runtime.NumCPU()
	workqueue.ParallelizeUntil(context.Background(), numCPU, len(allNodes), countPodsForNode)

	return &pfs, nil
}

// PreFilter is called at the beginning of the scheduling cycle. All PreFilter
// plugins must return success or the pod will be rejected.
func (wp *WorkloadPolicy) PreFilter(ctx context.Context, state *framework.CycleState,
	pod *v1.Pod) *framework.Status {
	// ignore daemonSet pod as it will be scheduled by the daemonSet controller
	// ignore pods without workload-policy labels
	if helpers.IsDaemonSetPod(pod) || !helpers.HasWorkloadPolicyLabel(pod) {
		return nil
	}

	pfs, err := wp.calculatePreFilterState(pod)
	if err != nil {
		return framework.AsStatus(err)
	}

	state.Write(preFilterStateKey, pfs)
	return nil
}

// PreFilterExtensions returns prefilter extensions, pod add and remove.
func (wp *WorkloadPolicy) PreFilterExtensions() framework.PreFilterExtensions {
	return wp
}

// getPreFilterState fetches a pre-computed preFilterState.
func getPreFilterState(cycleState *framework.CycleState) (*PreFilterState, error) {
	cs, err := cycleState.Read(preFilterStateKey)
	if err != nil {
		// preFilterState doesn't exist, likely PreFilter wasn't invoked.
		return nil, fmt.Errorf("reading %q from cycleState: %v", preFilterStateKey, err)
	}

	pfs, ok := cs.(*PreFilterState)
	if !ok {
		return nil, fmt.Errorf("%+v convert to preFilterState error", cs)
	}

	return pfs, nil
}

// Add/Subtract 1 from pre-computed data in cycleState.
func (pfs *PreFilterState) updatePreFilterStateWithPod(updatedPod, preemptPod *v1.Pod, node *v1.Node, delta int32) {
	if pfs == nil || node == nil || updatedPod.Namespace != preemptPod.Namespace {
		return
	}

	tpKey := pfs.Constraint.TopologyKey
	tpVal, ok := node.Labels[tpKey]
	if !ok {
		return
	}

	podLabelSet := labels.Set(updatedPod.Labels)
	if !pfs.Constraint.Selector.Matches(podLabelSet) {
		return
	}

	tpPair := helpers.TopologyPair{
		Key:   tpKey,
		Value: tpVal,
	}
	*pfs.TpPairToMatchNum[tpPair] += delta
}

// AddPod from pre-computed data in cycleState.
func (wp *WorkloadPolicy) AddPod(ctx context.Context, cycleState *framework.CycleState, podToSchedule *v1.Pod, podInfoToAdd *framework.PodInfo, nodeInfo *framework.NodeInfo) *framework.Status {
	// ignore daemonSet pod as it will be scheduled by the daemonSet controller
	// ignore pods without workload-policy labels
	if helpers.IsDaemonSetPod(podToSchedule) || !helpers.HasWorkloadPolicyLabel(podToSchedule) {
		return framework.NewStatus(framework.Success, "")
	}

	pfs, err := getPreFilterState(cycleState)
	if err != nil {
		return framework.AsStatus(err)
	}

	pfs.updatePreFilterStateWithPod(podInfoToAdd.Pod, podToSchedule, nodeInfo.Node(), 1)

	return framework.NewStatus(framework.Success, "")
}

// RemovePod from pre-computed data in cycleState.
func (wp *WorkloadPolicy) RemovePod(ctx context.Context, cycleState *framework.CycleState, podToSchedule *v1.Pod, podInfoToRemove *framework.PodInfo, nodeInfo *framework.NodeInfo) *framework.Status {
	// ignore daemonSet pod as it will be scheduled by the daemonSet controller
	// ignore pods without workload-policy labels
	if helpers.IsDaemonSetPod(podToSchedule) || !helpers.HasWorkloadPolicyLabel(podToSchedule) {
		return framework.NewStatus(framework.Success, "")
	}

	pfs, err := getPreFilterState(cycleState)
	if err != nil {
		return framework.AsStatus(err)
	}

	pfs.updatePreFilterStateWithPod(podInfoToRemove.Pod, podToSchedule, nodeInfo.Node(), -1)

	return framework.NewStatus(framework.Success, "")
}

func (wp *WorkloadPolicy) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	nodeInfo *framework.NodeInfo) *framework.Status {
	// ignore daemonSet pod as it will be scheduled by the daemonSet controller
	// ignore pods without workload-policy labels
	if helpers.IsDaemonSetPod(pod) || !helpers.HasWorkloadPolicyLabel(pod) {
		return framework.NewStatus(framework.Success, "")
	}

	node := nodeInfo.Node()
	if node == nil {
		return framework.AsStatus(fmt.Errorf("node not found"))
	}

	pfs, err := getPreFilterState(state)
	if err != nil {
		return framework.AsStatus(err)
	}

	wpName := pod.Labels[helpers.WorkloadPolicyLabelKey]
	tsp, err := wp.policyLister.WorkloadPolicies(pod.Namespace).Get(wpName)
	if err != nil {
		return framework.AsStatus(fmt.Errorf("failed to obtaining pod's workload-policy(%v): %v", wpName, err))
	}

	podLabelSet := labels.Set(pod.Labels)
	if !pfs.Constraint.Selector.Matches(podLabelSet) {
		klog.ErrorS(err, "The labelSelector set in the pod's workload-policy(%v) does not match %v", wpName, pfs.Constraint.Selector)
		return framework.NewStatus(framework.Unschedulable, helpers.ErrReasonConstraintsNotMatch)
	}

	tpKey := tsp.Spec.TopologyKey
	tpVal, ok := node.Labels[tpKey]
	if !ok {
		klog.V(5).Infof("node '%s' doesn't have required label '%s'", node.Name, tpKey)
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, helpers.ErrReasonNodeLabelNotMatch)
	}

	check, err := helpers.CheckTopologyValueReached(tpKey, tpVal, pfs.Constraint.AllocationPolicy, pfs.TpPairToMatchNum)
	if err != nil {
		return framework.NewStatus(framework.Unschedulable, err.Error())
	} else if !check || pfs.AllocationType == helpers.AllocationTypePreferred {
		return nil
	} else if pfs.AllocationType == helpers.AllocationTypeRequired {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, helpers.ErrReasonReachReplicas)
	}
	return framework.NewStatus(framework.Unschedulable, helpers.ErrReasonConstraintsNotMatch)
}

func getNodeScore(count, desired float64, policy string) int64 {
	if policy == helpers.AllocationMethodBalance {
		return int64((1. - count/desired) * float64(framework.MaxNodeScore))
	}

	if policy == helpers.AllocationMethodFill {
		return int64(count / desired * float64(framework.MaxNodeScore))
	}
	return 0
}

func (wp *WorkloadPolicy) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	nodeName string) (int64, *framework.Status) {
	// ignore daemonSet pod as it will be scheduled by the daemonSet controller
	// ignore pods without workload-policy labels
	if helpers.IsDaemonSetPod(pod) || !helpers.HasWorkloadPolicyLabel(pod) {
		return 0, framework.NewStatus(framework.Success, "")
	}

	pfs, err := getPreFilterState(state)
	if err != nil || pfs == nil {
		return 0, framework.NewStatus(framework.Success, "")
	}

	node, err := wp.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil || node == nil || node.Node() == nil {
		return 0, framework.NewStatus(framework.Error, "node(%v) not exist", nodeName)
	}

	// check node labels
	tpKey := pfs.Constraint.TopologyKey
	tpVal, ok := node.Node().Labels[tpKey]
	if !ok {
		return 0, framework.NewStatus(framework.Success, "")
	}

	pair := helpers.TopologyPair{
		Key:   tpKey,
		Value: tpVal,
	}

	desired, ok := pfs.Constraint.AllocationPolicy[tpVal]
	if !ok {
		return 0, framework.NewStatus(framework.Success, "")
	}

	count, ok := pfs.TpPairToMatchNum[pair]
	if count == nil || !ok {
		return 0, framework.NewStatus(framework.Success, "")
	}

	// when count >= desired and AllocationType  == Required, return directly
	if *count >= desired && pfs.AllocationType == helpers.AllocationTypeRequired {
		return 0, framework.NewStatus(framework.Success, "")
	}

	return getNodeScore(float64(*count), float64(desired), pfs.AllocationMethod),
		framework.NewStatus(framework.Success, "")
}

func (wp *WorkloadPolicy) ScoreExtensions() framework.ScoreExtensions {
	return wp
}

func (wp *WorkloadPolicy) NormalizeScore(ctx context.Context, state *framework.CycleState, p *v1.Pod,
	scores framework.NodeScoreList) *framework.Status {
	return framework.NewStatus(framework.Success)
}

func (wp *WorkloadPolicy) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	nodeName string) *framework.Status {
	// ignore daemonSet pod as it will be scheduled by the daemonSet controller
	// ignore pods without workload-policy labels
	if helpers.IsDaemonSetPod(pod) || !helpers.HasWorkloadPolicyLabel(pod) {
		return framework.NewStatus(framework.Success, "")
	}

	pfs, err := getPreFilterState(state)
	if err != nil {
		return framework.AsStatus(err)
	}

	node, err := wp.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return framework.AsStatus(err)
	}

	pfs.updatePreFilterStateWithPod(pod, pod, node.Node(), 1)
	return framework.NewStatus(framework.Success, "")
}

func (wp *WorkloadPolicy) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	// ignore daemonSet pod as it will be scheduled by the daemonSet controller
	// ignore pods without workload-policy labels
	if helpers.IsDaemonSetPod(pod) || !helpers.HasWorkloadPolicyLabel(pod) {
		return
	}

	pfs, err := getPreFilterState(state)
	if err != nil {
		return
	}

	node, err := wp.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return
	}
	pfs.updatePreFilterStateWithPod(pod, pod, node.Node(), -1)
}
