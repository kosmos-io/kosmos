// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// DistributionPolicyLister helps list DistributionPolicies.
// All objects returned here must be treated as read-only.
type DistributionPolicyLister interface {
	// List lists all DistributionPolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.DistributionPolicy, err error)
	// DistributionPolicies returns an object that can list and get DistributionPolicies.
	DistributionPolicies(namespace string) DistributionPolicyNamespaceLister
	DistributionPolicyListerExpansion
}

// distributionPolicyLister implements the DistributionPolicyLister interface.
type distributionPolicyLister struct {
	indexer cache.Indexer
}

// NewDistributionPolicyLister returns a new DistributionPolicyLister.
func NewDistributionPolicyLister(indexer cache.Indexer) DistributionPolicyLister {
	return &distributionPolicyLister{indexer: indexer}
}

// List lists all DistributionPolicies in the indexer.
func (s *distributionPolicyLister) List(selector labels.Selector) (ret []*v1alpha1.DistributionPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.DistributionPolicy))
	})
	return ret, err
}

// DistributionPolicies returns an object that can list and get DistributionPolicies.
func (s *distributionPolicyLister) DistributionPolicies(namespace string) DistributionPolicyNamespaceLister {
	return distributionPolicyNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// DistributionPolicyNamespaceLister helps list and get DistributionPolicies.
// All objects returned here must be treated as read-only.
type DistributionPolicyNamespaceLister interface {
	// List lists all DistributionPolicies in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.DistributionPolicy, err error)
	// Get retrieves the DistributionPolicy from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.DistributionPolicy, error)
	DistributionPolicyNamespaceListerExpansion
}

// distributionPolicyNamespaceLister implements the DistributionPolicyNamespaceLister
// interface.
type distributionPolicyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all DistributionPolicies in the indexer for a given namespace.
func (s distributionPolicyNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.DistributionPolicy, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.DistributionPolicy))
	})
	return ret, err
}

// Get retrieves the DistributionPolicy from the indexer for a given namespace and name.
func (s distributionPolicyNamespaceLister) Get(name string) (*v1alpha1.DistributionPolicy, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("distributionpolicy"), name)
	}
	return obj.(*v1alpha1.DistributionPolicy), nil
}
