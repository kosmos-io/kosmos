package validators

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

// VirtualClusterValidator validates VirtualCluster resources
type VirtualClusterValidator struct{}

// ValidateCreate validates VirtualCluster during CREATE operation
func (v *VirtualClusterValidator) ValidateCreate(_ context.Context, obj runtime.Object) error {
	virtualCluster, ok := obj.(*v1alpha1.VirtualCluster)
	if !ok {
		return fmt.Errorf("expected a VirtualCluster object but got %T", obj)
	}

	// Validate PromotePolicies: NodeCount > 0
	for _, policy := range virtualCluster.Spec.PromotePolicies {
		if policy.NodeCount <= 0 {
			return fmt.Errorf("PromotePolicy NodeCount must be greater than 0, found %d", policy.NodeCount)
		}
	}
	return nil
}

// ValidateUpdate validates VirtualCluster during UPDATE operation
func (v *VirtualClusterValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) error {
	klog.Info("Starting ValidateUpdate for VirtualCluster")

	oldVirtualCluster, ok := oldObj.(*v1alpha1.VirtualCluster)
	if !ok {
		err := fmt.Errorf("expected an old VirtualCluster object but got %T", oldObj)
		klog.Error(err)
		return err
	}

	newVirtualCluster, ok := newObj.(*v1alpha1.VirtualCluster)
	if !ok {
		err := fmt.Errorf("expected a new VirtualCluster object but got %T", newObj)
		klog.Error(err)
		return err
	}

	// If the object is being deleted, skip validation
	if !newVirtualCluster.DeletionTimestamp.IsZero() {
		klog.Info("VirtualCluster is being deleted, skipping validation")
		return nil
	}

	klog.Infof("Old VirtualCluster: %+v", oldVirtualCluster.Spec.PromoteResources.NodeInfos)
	klog.Infof("New VirtualCluster: %+v", newVirtualCluster.Spec.PromoteResources.NodeInfos)

	// Check if NodeInfos has been modified
	if !reflect.DeepEqual(oldVirtualCluster.Spec.PromoteResources.NodeInfos, newVirtualCluster.Spec.PromoteResources.NodeInfos) {
		klog.Info("Detected modification in NodeInfos, validating against NodeCount")

		// Compute the total NodeCount from PromotePolicies
		nodeCount := int32(0)
		for _, policy := range newVirtualCluster.Spec.PromotePolicies {
			nodeCount += policy.NodeCount
		}

		klog.Infof("Computed NodeCount from PromotePolicies: %d", nodeCount)
		klog.Infof("NodeInfos count in new VirtualCluster: %d", len(newVirtualCluster.Spec.PromoteResources.NodeInfos))

		// Validate NodeInfos count matches NodeCount
		if int32(len(newVirtualCluster.Spec.PromoteResources.NodeInfos)) != nodeCount {
			err := fmt.Errorf("mismatch between NodeInfos count (%d) and total NodeCount (%d)",
				len(newVirtualCluster.Spec.PromoteResources.NodeInfos), nodeCount)
			klog.Error(err)
			return err
		}
	} else {
		klog.Info("No changes detected in NodeInfos, skipping validation")
	}

	klog.Info("ValidateUpdate for VirtualCluster completed successfully")
	return nil
}

// ValidateDelete validates VirtualCluster during DELETE operation
func (v *VirtualClusterValidator) ValidateDelete(_ context.Context, _ runtime.Object) error {
	// Allow all DELETE operations without validation
	return nil
}
