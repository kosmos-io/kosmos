// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	internalinterfaces "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// ClusterDistributionPolicies returns a ClusterDistributionPolicyInformer.
	ClusterDistributionPolicies() ClusterDistributionPolicyInformer
	// DistributionPolicies returns a DistributionPolicyInformer.
	DistributionPolicies() DistributionPolicyInformer
	// WorkloadPolicies returns a WorkloadPolicyInformer.
	WorkloadPolicies() WorkloadPolicyInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// ClusterDistributionPolicies returns a ClusterDistributionPolicyInformer.
func (v *version) ClusterDistributionPolicies() ClusterDistributionPolicyInformer {
	return &clusterDistributionPolicyInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// DistributionPolicies returns a DistributionPolicyInformer.
func (v *version) DistributionPolicies() DistributionPolicyInformer {
	return &distributionPolicyInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// WorkloadPolicies returns a WorkloadPolicyInformer.
func (v *version) WorkloadPolicies() WorkloadPolicyInformer {
	return &workloadPolicyInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
