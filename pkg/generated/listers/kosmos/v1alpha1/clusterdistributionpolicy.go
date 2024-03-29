// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ClusterDistributionPolicyLister helps list ClusterDistributionPolicies.
// All objects returned here must be treated as read-only.
type ClusterDistributionPolicyLister interface {
	// List lists all ClusterDistributionPolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ClusterDistributionPolicy, err error)
	// Get retrieves the ClusterDistributionPolicy from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ClusterDistributionPolicy, error)
	ClusterDistributionPolicyListerExpansion
}

// clusterDistributionPolicyLister implements the ClusterDistributionPolicyLister interface.
type clusterDistributionPolicyLister struct {
	indexer cache.Indexer
}

// NewClusterDistributionPolicyLister returns a new ClusterDistributionPolicyLister.
func NewClusterDistributionPolicyLister(indexer cache.Indexer) ClusterDistributionPolicyLister {
	return &clusterDistributionPolicyLister{indexer: indexer}
}

// List lists all ClusterDistributionPolicies in the indexer.
func (s *clusterDistributionPolicyLister) List(selector labels.Selector) (ret []*v1alpha1.ClusterDistributionPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ClusterDistributionPolicy))
	})
	return ret, err
}

// Get retrieves the ClusterDistributionPolicy from the index for a given name.
func (s *clusterDistributionPolicyLister) Get(name string) (*v1alpha1.ClusterDistributionPolicy, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("clusterdistributionpolicy"), name)
	}
	return obj.(*v1alpha1.ClusterDistributionPolicy), nil
}
