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
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/scheduler/framework/plugins/volumebinding/volume_binding.go

package leafnodevolumebinding

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/component-helpers/storage/ephemeral"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	scheduling "k8s.io/kubernetes/pkg/scheduler/framework/plugins/volumebinding"

	"github.com/kosmos.io/kosmos/pkg/apis/config"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/helpers"
)

const (
	stateKey framework.StateKey = Name
)

// the state is initialized in PreFilter phase. because we save the pointer in
// framework.CycleState, in the later phases we don't need to call Write method
// to update the value
type stateData struct {
	skip         bool // set true if pod does not have PVCs
	boundClaims  []*corev1.PersistentVolumeClaim
	claimsToBind []*corev1.PersistentVolumeClaim
	allBound     bool
	// podVolumesByNode holds the pod's volume information found in the Filter
	// phase for each node
	// it's initialized in the PreFilter phase
	podVolumesByNode map[string]*scheduling.PodVolumes
	sync.Mutex
}

func (d *stateData) Clone() framework.StateData {
	return d
}

// VolumeBinding is a plugin that binds pod volumes in scheduling.
// In the Filter phase, pod binding cache is created for the pod and used in
// Reserve and PreBind phases.
type VolumeBinding struct {
	Binder           scheduling.SchedulerVolumeBinder
	PVCLister        corelisters.PersistentVolumeClaimLister
	NodeLister       corelisters.NodeLister
	frameworkHandler framework.Handle
}

var _ framework.PreFilterPlugin = &VolumeBinding{}
var _ framework.FilterPlugin = &VolumeBinding{}
var _ framework.ReservePlugin = &VolumeBinding{}
var _ framework.PreBindPlugin = &VolumeBinding{}

// Name is the name of the plugin used in Registry and configurations.
const Name = "LeafNodeVolumeBinding"

// Name returns name of the plugin. It is used in logs, etc.
func (pl *VolumeBinding) Name() string {
	return Name
}

// podHasPVCs returns 2 values:
// - the first one to denote if the given "pod" has any PVC defined.
// - the second one to return any error if the requested PVC is illegal.
func (pl *VolumeBinding) podHasPVCs(pod *corev1.Pod) (bool, error) {
	hasPVC := false
	for i, vol := range pod.Spec.Volumes {
		var pvcName string
		isEphemeral := false
		switch {
		case vol.PersistentVolumeClaim != nil:
			pvcName = vol.PersistentVolumeClaim.ClaimName
		case vol.Ephemeral != nil:
			pvcName = ephemeral.VolumeClaimName(pod, &pod.Spec.Volumes[i])
			isEphemeral = true
		default:
			// Volume is not using a PVC, ignore
			continue
		}
		hasPVC = true
		pvc, err := pl.PVCLister.PersistentVolumeClaims(pod.Namespace).Get(pvcName)
		if err != nil {
			// The error usually has already enough context ("persistentvolumeclaim "myclaim" not found"),
			// but we can do better for generic ephemeral inline volumes where that situation
			// is normal directly after creating a pod.
			if isEphemeral && apierrors.IsNotFound(err) {
				err = fmt.Errorf("waiting for ephemeral volume controller to create the persistentvolumeclaim %q", pvcName)
			}
			return hasPVC, err
		}

		if pvc.Status.Phase == corev1.ClaimLost {
			return hasPVC, fmt.Errorf("persistentvolumeclaim %q bound to non-existent persistentvolume %q", pvc.Name, pvc.Spec.VolumeName)
		}
		if pvc.DeletionTimestamp != nil {
			return hasPVC, fmt.Errorf("persistentvolumeclaim %q is being deleted", pvc.Name)
		}

		if isEphemeral {
			if err := ephemeral.VolumeIsForPod(pod, pvc); err != nil {
				return hasPVC, err
			}
		}
	}
	return hasPVC, nil
}

// PreFilter invoked at the prefilter extension point to check if pod has all
// immediate PVCs bound. If not all immediate PVCs are bound, an
// UnschedulableAndUnresolvable is returned.
func (pl *VolumeBinding) PreFilter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod) (*framework.PreFilterResult, *framework.Status) {
	// If pod does not reference any PVC, we don't need to do anything.
	if hasPVC, err := pl.podHasPVCs(pod); err != nil {
		return nil, framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
	} else if !hasPVC {
		state.Write(stateKey, &stateData{skip: true})
		return nil, nil
	}
	boundClaims, claimsToBind, unboundClaimsImmediate, err := pl.Binder.GetPodVolumes(pod)
	if err != nil {
		return nil, framework.AsStatus(err)
	}
	if len(unboundClaimsImmediate) > 0 {
		// Return UnschedulableAndUnresolvable error if immediate claims are
		// not bound. Pod will be moved to active/backoff queues once these
		// claims are bound by PV controller.
		status := framework.NewStatus(framework.UnschedulableAndUnresolvable)
		status.AppendReason("pod has unbound immediate PersistentVolumeClaims")
		return nil, status
	}
	state.Write(stateKey, &stateData{boundClaims: boundClaims, claimsToBind: claimsToBind, podVolumesByNode: make(map[string]*scheduling.PodVolumes)})
	return nil, nil
}

// PreFilterExtensions returns prefilter extensions, pod add and remove.
func (pl *VolumeBinding) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func getStateData(cs *framework.CycleState) (*stateData, error) {
	state, err := cs.Read(stateKey)
	if err != nil {
		return nil, err
	}
	s, ok := state.(*stateData)
	if !ok {
		return nil, errors.New("unable to convert state into stateData")
	}
	return s, nil
}

// Filter invoked at the filter extension point.
// It evaluates if a pod can fit due to the volumes it requests,
// for both bound and unbound PVCs.
//
// For PVCs that are bound, then it checks that the corresponding PV's node affinity is
// satisfied by the given node.
//
// For PVCs that are unbound, it tries to find available PVs that can satisfy the PVC requirements
// and that the PV node affinity is satisfied by the given node.
//
// If storage capacity tracking is enabled, then enough space has to be available
// for the node and volumes that still need to be created.
//
// The predicate returns true if all bound PVCs have compatible PVs with the node, and if all unbound
// PVCs can be matched with an available and node-compatible PV.
func (pl *VolumeBinding) Filter(_ context.Context, cs *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	if helpers.HasLeafNodeTaint(node) {
		return nil
	}

	state, err := getStateData(cs)
	if err != nil {
		return framework.AsStatus(err)
	}

	if state.skip {
		return nil
	}

	podVolumes, reasons, err := pl.Binder.FindPodVolumes(pod, state.boundClaims, state.claimsToBind, node)

	if err != nil {
		return framework.AsStatus(err)
	}

	if len(reasons) > 0 {
		status := framework.NewStatus(framework.UnschedulableAndUnresolvable)
		for _, reason := range reasons {
			status.AppendReason(string(reason))
		}
		return status
	}

	// multiple goroutines call `Filter` on different nodes simultaneously and the `CycleState` may be duplicated, so we must use a local lock here
	state.Lock()
	state.podVolumesByNode[node.Name] = podVolumes
	state.Unlock()
	return nil
}

// Reserve reserves volumes of pod and saves binding status in cycle state.
func (pl *VolumeBinding) Reserve(_ context.Context, cs *framework.CycleState, pod *corev1.Pod, nodeName string) *framework.Status {
	node, err := pl.NodeLister.Get(nodeName)
	if err != nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	if helpers.HasLeafNodeTaint(node) {
		return nil
	}

	state, err := getStateData(cs)
	if err != nil {
		return framework.AsStatus(err)
	}
	// we don't need to hold the lock as only one node will be reserved for the given pod
	podVolumes, ok := state.podVolumesByNode[nodeName]
	if ok {
		allBound, err := pl.Binder.AssumePodVolumes(pod, nodeName, podVolumes)
		if err != nil {
			return framework.AsStatus(err)
		}
		state.allBound = allBound
	} else {
		// may not exist if the pod does not reference any PVC
		state.allBound = true
	}
	return nil
}

// PreBind will make the API update with the assumed bindings and wait until
// the PV controller has completely finished the binding operation.
//
// If binding errors, times out or gets undone, then an error will be returned to
// retry scheduling.
func (pl *VolumeBinding) PreBind(ctx context.Context, cs *framework.CycleState, pod *corev1.Pod, nodeName string) *framework.Status {
	node, err := pl.NodeLister.Get(nodeName)
	if err != nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	if helpers.HasLeafNodeTaint(node) {
		return nil
	}

	s, err := getStateData(cs)
	if err != nil {
		return framework.AsStatus(err)
	}
	if s.allBound {
		// no need to bind volumes
		return nil
	}
	// we don't need to hold the lock as only one node will be pre-bound for the given pod
	podVolumes, ok := s.podVolumesByNode[nodeName]
	if !ok {
		return framework.AsStatus(fmt.Errorf("no pod volumes found for node %q", nodeName))
	}
	klog.V(5).InfoS("Trying to bind volumes for pod", "pod", klog.KObj(pod))
	err = pl.Binder.BindPodVolumes(ctx, pod, podVolumes)
	if err != nil {
		klog.V(1).InfoS("Failed to bind volumes for pod", "pod", klog.KObj(pod), "err", err)
		return framework.AsStatus(err)
	}
	klog.V(5).InfoS("Success binding volumes for pod", "pod", klog.KObj(pod))
	return nil
}

// Unreserve clears assumed PV and PVC cache.
// It's idempotent, and does nothing if no cache found for the given pod.
func (pl *VolumeBinding) Unreserve(_ context.Context, cs *framework.CycleState, _ *corev1.Pod, nodeName string) {
	node, err := pl.NodeLister.Get(nodeName)
	if err != nil {
		return
	}

	if helpers.HasLeafNodeTaint(node) {
		return
	}

	s, err := getStateData(cs)
	if err != nil {
		return
	}
	// we don't need to hold the lock as only one node may be unreserved
	podVolumes, ok := s.podVolumesByNode[nodeName]
	if !ok {
		return
	}
	pl.Binder.RevertAssumedPodVolumes(podVolumes)
}

// New initializes a new plugin and returns it.
func New(plArgs runtime.Object, fh framework.Handle) (framework.Plugin, error) {
	args, ok := plArgs.(*config.LeafNodeVolumeBindingArgs)
	if !ok {
		return nil, fmt.Errorf("want args to be of type VolumeBindingArgs, got %T", plArgs)
	}

	podInformer := fh.SharedInformerFactory().Core().V1().Pods()
	nodeInformer := fh.SharedInformerFactory().Core().V1().Nodes()
	pvcInformer := fh.SharedInformerFactory().Core().V1().PersistentVolumeClaims()
	pvInformer := fh.SharedInformerFactory().Core().V1().PersistentVolumes()
	storageClassInformer := fh.SharedInformerFactory().Storage().V1().StorageClasses()
	csiNodeInformer := fh.SharedInformerFactory().Storage().V1().CSINodes()
	capacityCheck := scheduling.CapacityCheck{
		CSIDriverInformer:          fh.SharedInformerFactory().Storage().V1().CSIDrivers(),
		CSIStorageCapacityInformer: fh.SharedInformerFactory().Storage().V1().CSIStorageCapacities(),
	}
	binder := scheduling.NewVolumeBinder(fh.ClientSet(), podInformer, nodeInformer, csiNodeInformer, pvcInformer, pvInformer, storageClassInformer, capacityCheck, time.Duration(args.BindTimeoutSeconds)*time.Second)

	return &VolumeBinding{
		Binder:           binder,
		PVCLister:        pvcInformer.Lister(),
		NodeLister:       nodeInformer.Lister(),
		frameworkHandler: fh,
	}, nil
}
