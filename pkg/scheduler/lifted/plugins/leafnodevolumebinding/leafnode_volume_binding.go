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
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/storage/ephemeral"
	"k8s.io/component-helpers/storage/volume"
	"k8s.io/klog/v2"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	scheduling "k8s.io/kubernetes/pkg/scheduler/framework/plugins/volumebinding"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/volumebinding/metrics"

	"github.com/kosmos.io/kosmos/pkg/apis/config"
	"github.com/kosmos.io/kosmos/pkg/scheduler/lifted/helpers"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	stateKey framework.StateKey = Name
	nodeMode                    = "leafNodeMode"
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
	PVLister         corelisters.PersistentVolumeLister
	NodeLister       corelisters.NodeLister
	SCLister         storagelisters.StorageClassLister
	frameworkHandler framework.Handle
	pvCache          scheduling.PVAssumeCache
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
// UnscheduledAndUnresolvable is returned.
func (pl *VolumeBinding) PreFilter(_ context.Context, state *framework.CycleState, pod *corev1.Pod) (*framework.PreFilterResult, *framework.Status) {
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
// nolint:revive
func (pl *VolumeBinding) Filter(_ context.Context, cs *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	state, err := getStateData(cs)
	if err != nil {
		return framework.AsStatus(err)
	}

	if state.skip {
		return nil
	}

	podVolumes := &scheduling.PodVolumes{}
	reasons := scheduling.ConflictReasons{}
	isKosmosClusterNode := false

	if utils.HasKosmosNodeLabel(node) {
		// the node is a kosmos node
		if cluster, ok := node.Annotations[nodeMode]; ok && cluster == "one2cluster" {
			// it is in "one2cluster" mode
			isKosmosClusterNode = true
			_, reasons, err = pl.FindPodKosmosVolumes(pod, state.boundClaims, state.claimsToBind, node)
		} else {
			// it is in "one2one" mode
			if len(state.boundClaims) <= 0 {
				// the pod is newly created
				return nil
			} else {
				// the pod has been created and is now being restarted.
				podVolumes, reasons, err = pl.Binder.FindPodVolumes(pod, state.boundClaims, state.claimsToBind, node)
			}
		}
	} else {
		// the node is real node
		podVolumes, reasons, err = pl.Binder.FindPodVolumes(pod, state.boundClaims, state.claimsToBind, node)
	}

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

	// this operation is not required for kosmos node
	if !isKosmosClusterNode {
		// multiple goroutines call `Filter` on different nodes simultaneously and the `CycleState` may be duplicated, so we must use a local lock here
		state.Lock()
		state.podVolumesByNode[node.Name] = podVolumes
		state.Unlock()
	}

	return nil
}

// FindPodKosmosVolumes finds the matching PVs for PVCs and kosmos nodes to provision PVs
// for the given pod and node. If the kosmos node does not fit, confilict reasons are
// returned.
func (pl *VolumeBinding) FindPodKosmosVolumes(pod *corev1.Pod, boundClaims, claimsToBind []*corev1.PersistentVolumeClaim, node *corev1.Node) (podVolumes *PodVolumes, reasons scheduling.ConflictReasons, err error) {
	podVolumes = &PodVolumes{}

	// Warning: Below log needs high verbosity as it can be printed several times (#60933).
	klog.V(5).InfoS("FindPodKosmosVolumes", "pod", klog.KObj(pod), "node", klog.KObj(node))

	// Initialize to true for pods that don't have volumes. These
	// booleans get translated into reason strings when the function
	// returns without an error.
	unboundVolumesSatisfied := true
	boundVolumesSatisfied := true
	sufficientStorage := true
	boundPVsFound := true
	defer func() {
		if err != nil {
			return
		}
		if !boundVolumesSatisfied {
			reasons = append(reasons, scheduling.ErrReasonNodeConflict)
		}
		if !unboundVolumesSatisfied {
			reasons = append(reasons, scheduling.ErrReasonBindConflict)
		}
		if !sufficientStorage {
			reasons = append(reasons, scheduling.ErrReasonNotEnoughSpace)
		}
		if !boundPVsFound {
			reasons = append(reasons, scheduling.ErrReasonPVNotExist)
		}
	}()

	defer func() {
		if err != nil {
			metrics.VolumeSchedulingStageFailed.WithLabelValues("predicate").Inc()
		}
	}()

	var (
		staticBindings    []*BindingInfo
		dynamicProvisions []*corev1.PersistentVolumeClaim
	)
	defer func() {
		// Although we do not distinguish nil from empty in this function, for
		// easier testing, we normalize empty to nil.
		if len(staticBindings) == 0 {
			staticBindings = nil
		}
		if len(dynamicProvisions) == 0 {
			dynamicProvisions = nil
		}
		podVolumes.StaticBindings = staticBindings
		podVolumes.DynamicProvisions = dynamicProvisions
	}()

	// Check PV node affinity on bound volumes
	if len(boundClaims) > 0 {
		boundVolumesSatisfied, boundPVsFound, err = pl.checkBoundClaims(boundClaims, node, pod)
		if err != nil {
			return
		}
	}

	// Find matching volumes and node for unbound claims
	if len(claimsToBind) > 0 {
		var (
			claimsToFindMatching []*corev1.PersistentVolumeClaim
			claimsToProvision    []*corev1.PersistentVolumeClaim
		)

		// Filter out claims to provision
		for _, claim := range claimsToBind {
			if clusterOwners, ok := claim.Annotations[utils.KosmosResourceOwnersAnnotations]; ok {
				if clusterOwners != node.Annotations[utils.KosmosNodeOwnedByClusterAnnotations] {
					// Fast path, skip unmatched node.
					unboundVolumesSatisfied = false
					return
				}
				claimsToProvision = append(claimsToProvision, claim)
			} else {
				claimsToFindMatching = append(claimsToFindMatching, claim)
			}
		}

		// Find matching volumes
		if len(claimsToFindMatching) > 0 {
			var unboundClaims []*corev1.PersistentVolumeClaim
			unboundVolumesSatisfied, staticBindings, unboundClaims, err = pl.findMatchingVolumes(pod, claimsToFindMatching, node)
			if err != nil {
				return
			}
			claimsToProvision = append(claimsToProvision, unboundClaims...)
		}

		// Check for claims to provision. This is the first time where we potentially
		// find out that storage is not sufficient for the node.
		if len(claimsToProvision) > 0 {
			unboundVolumesSatisfied, sufficientStorage, dynamicProvisions, err = pl.checkVolumeProvisions(pod, claimsToProvision, node)
			if err != nil {
				return
			}
		}
	}

	return
}

// checkVolumeProvisions checks given unbound claims (the claims have gone through func
// findMatchingVolumes, and do not have matching volumes for binding), and return true
// if all the claims are eligible for dynamic provision.
func (pl *VolumeBinding) checkVolumeProvisions(pod *corev1.Pod, claimsToProvision []*corev1.PersistentVolumeClaim, node *corev1.Node) (provisionSatisfied, sufficientStorage bool, dynamicProvisions []*corev1.PersistentVolumeClaim, err error) {
	dynamicProvisions = []*corev1.PersistentVolumeClaim{}

	// We return early with provisionedClaims == nil if a check
	// fails or we encounter an error.
	for _, claim := range claimsToProvision {
		pvcName := getPVCName(claim)
		className := volume.GetPersistentVolumeClaimClass(claim)
		if className == "" {
			return false, false, nil, fmt.Errorf("no class for claim %q", pvcName)
		}

		class, err := pl.SCLister.Get(className)
		if err != nil {
			return false, false, nil, fmt.Errorf("failed to find storage class %q", className)
		}
		provisioner := class.Provisioner
		if provisioner == "" || provisioner == volume.NotSupportedProvisioner {
			klog.V(4).InfoS("Storage class of claim does not support dynamic provisioning", "storageClassName", className, "PVC", klog.KObj(claim))
			return false, true, nil, nil
		}

		// Check if the node can satisfy the topology requirement in the class
		if !v1helper.MatchTopologySelectorTerms(class.AllowedTopologies, labels.Set(node.Labels)) {
			klog.V(4).InfoS("Node cannot satisfy provisioning topology requirements of claim", "node", klog.KObj(node), "PVC", klog.KObj(claim))
			return false, true, nil, nil
		}

		dynamicProvisions = append(dynamicProvisions, claim)

	}
	klog.V(4).InfoS("Provisioning for claims of pod that has no matching volumes...", "claimCount", len(claimsToProvision), "pod", klog.KObj(pod), "node", klog.KObj(node))

	return true, true, dynamicProvisions, nil
}

func getPVCName(pvc *corev1.PersistentVolumeClaim) string {
	return pvc.Namespace + "/" + pvc.Name
}

// findMatchingVolumes tries to find matching volumes for given claims,
// and return unbound claims for further provision.
func (pl *VolumeBinding) findMatchingVolumes(pod *corev1.Pod, claimsToBind []*corev1.PersistentVolumeClaim, node *corev1.Node) (foundMatches bool, bindings []*BindingInfo, unboundClaims []*corev1.PersistentVolumeClaim, err error) {
	// Sort all the claims by increasing size request to get the smallest fits
	sort.Sort(byPVCSize(claimsToBind))

	chosenPVs := map[string]*corev1.PersistentVolume{}

	foundMatches = true

	for _, pvc := range claimsToBind {
		// Get storage class name from each PVC
		storageClassName := volume.GetPersistentVolumeClaimClass(pvc)
		allPVs := pl.pvCache.ListPVs(storageClassName)

		// Find a matching PV
		pv, err := volume.FindMatchingVolume(pvc, allPVs, node, chosenPVs, true)
		if err != nil {
			return false, nil, nil, err
		}
		if pv == nil {
			klog.V(4).InfoS("No matching volumes for pod", "pod", klog.KObj(pod), "PVC", klog.KObj(pvc), "node", klog.KObj(node))
			unboundClaims = append(unboundClaims, pvc)
			foundMatches = false
			continue
		}

		// matching PV needs to be excluded so we don't select it again
		chosenPVs[pv.Name] = pv
		bindings = append(bindings, &BindingInfo{pvc: pvc, pv: pv})
		klog.V(5).InfoS("Found matching PV for PVC for pod", "PV", klog.KObj(pv), "PVC", klog.KObj(pvc), "node", klog.KObj(node), "pod", klog.KObj(pod))
	}

	if foundMatches {
		klog.V(4).InfoS("Found matching volumes for pod", "pod", klog.KObj(pod), "node", klog.KObj(node))
	}

	return
}

// checkBoundClaims Check whether the kosmos cluster owner annotation of the bound pvc/pvc matches the owner annotation of the node
func (pl *VolumeBinding) checkBoundClaims(claims []*corev1.PersistentVolumeClaim, node *corev1.Node, pod *corev1.Pod) (bool, bool, error) {
	for _, pvc := range claims {
		pvName := pvc.Spec.VolumeName
		pv, err := pl.PVLister.Get(pvName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				err = nil
			}
			return true, false, err
		}

		// todo Verification of migrated pods is not currently supported.
		// translator.IsPVMigratable(pv)

		err = pl.checkKosmosResourceOwner(pv, node)
		if err != nil {
			klog.V(4).InfoS("PersistentVolume and node mismatch for pod", "PV", klog.KRef("", pvName), "node", klog.KObj(node), "pod", klog.KObj(pod), "err", err)
			return false, true, nil
		}
		klog.V(5).InfoS("PersistentVolume and node matches for pod", "PV", klog.KRef("", pvName), "node", klog.KObj(node), "pod", klog.KObj(pod))
	}

	klog.V(4).InfoS("All bound volumes for pod match with node", "pod", klog.KObj(pod), "node", klog.KObj(node))
	return true, true, nil
}

// checkKosmosResourceOwner looks at the PV node affinity, and checks if the kosmos node has the same corresponding labels
// This ensures that we don't mount a volume that doesn't belong to this node
func (pl *VolumeBinding) checkKosmosResourceOwner(pv *corev1.PersistentVolume, node *corev1.Node) error {
	clusterOwners, ok := pv.Annotations[utils.KosmosResourceOwnersAnnotations]
	if !ok {
		// For pvc that has been bound to pv, but is not managed by kosmos
		err := CheckNodeAffinity(pv, node.Labels)
		return err
	}

	if !(clusterOwners == node.Annotations[utils.KosmosNodeOwnedByClusterAnnotations]) {
		klog.V(4).InfoS("This pv does not belong to the kosmos node", "node", klog.KObj(node), "pv", klog.KObj(pv))
		return fmt.Errorf("the owner cluster %s of the pv mismatch the owner cluster of %s this kosmos node", clusterOwners, node.Annotations[utils.KosmosNodeOwnedByClusterAnnotations])
	}

	return nil
}

// CheckNodeAffinity looks at the PV node affinity, and checks if the node has the same corresponding labels
// This ensures that we don't mount a volume that doesn't belong to this node
func CheckNodeAffinity(pv *corev1.PersistentVolume, nodeLabels map[string]string) error {
	if pv.Spec.NodeAffinity == nil {
		return fmt.Errorf("node Affinity not specified")
	}

	if pv.Spec.NodeAffinity.Required != nil {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels}}
		terms := pv.Spec.NodeAffinity.Required
		if matches, err := corev1helpers.MatchNodeSelectorTerms(node, terms); err != nil {
			return err
		} else if !matches {
			return fmt.Errorf("no matching NodeSelectorTerms")
		}
	}

	return nil
}

type byPVCSize []*corev1.PersistentVolumeClaim

func (a byPVCSize) Len() int {
	return len(a)
}

func (a byPVCSize) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a byPVCSize) Less(i, j int) bool {
	iSize := a[i].Spec.Resources.Requests[corev1.ResourceStorage]
	jSize := a[j].Spec.Resources.Requests[corev1.ResourceStorage]
	// return true if iSize is less than jSize
	return iSize.Cmp(jSize) == -1
}

// BindingInfo holds a binding between PV and PVC.
type BindingInfo struct {
	// PVC that needs to be bound
	pvc *corev1.PersistentVolumeClaim

	// Proposed PV to bind to this PVC
	pv *corev1.PersistentVolume
}

// PodVolumes holds pod's volumes information used in volume scheduling.
type PodVolumes struct {
	// StaticBindings are binding decisions for PVCs which can be bound to
	// pre-provisioned static PVs.
	StaticBindings []*BindingInfo
	// DynamicProvisions are PVCs that require dynamic provisioning
	DynamicProvisions []*corev1.PersistentVolumeClaim
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
		PVLister:         pvInformer.Lister(),
		SCLister:         storageClassInformer.Lister(),
		frameworkHandler: fh,
		pvCache:          scheduling.NewPVAssumeCache(pvInformer.Informer()),
	}, nil
}
