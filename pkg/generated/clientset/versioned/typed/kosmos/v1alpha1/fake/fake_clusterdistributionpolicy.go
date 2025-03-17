/*
Copyright The Kosmos Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterDistributionPolicies implements ClusterDistributionPolicyInterface
type FakeClusterDistributionPolicies struct {
	Fake *FakeKosmosV1alpha1
}

var clusterdistributionpoliciesResource = schema.GroupVersionResource{Group: "kosmos.io", Version: "v1alpha1", Resource: "clusterdistributionpolicies"}

var clusterdistributionpoliciesKind = schema.GroupVersionKind{Group: "kosmos.io", Version: "v1alpha1", Kind: "ClusterDistributionPolicy"}

// Get takes name of the clusterDistributionPolicy, and returns the corresponding clusterDistributionPolicy object, and an error if there is any.
func (c *FakeClusterDistributionPolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ClusterDistributionPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterdistributionpoliciesResource, name), &v1alpha1.ClusterDistributionPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterDistributionPolicy), err
}

// List takes label and field selectors, and returns the list of ClusterDistributionPolicies that match those selectors.
func (c *FakeClusterDistributionPolicies) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ClusterDistributionPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterdistributionpoliciesResource, clusterdistributionpoliciesKind, opts), &v1alpha1.ClusterDistributionPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ClusterDistributionPolicyList{ListMeta: obj.(*v1alpha1.ClusterDistributionPolicyList).ListMeta}
	for _, item := range obj.(*v1alpha1.ClusterDistributionPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterDistributionPolicies.
func (c *FakeClusterDistributionPolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterdistributionpoliciesResource, opts))
}

// Create takes the representation of a clusterDistributionPolicy and creates it.  Returns the server's representation of the clusterDistributionPolicy, and an error, if there is any.
func (c *FakeClusterDistributionPolicies) Create(ctx context.Context, clusterDistributionPolicy *v1alpha1.ClusterDistributionPolicy, opts v1.CreateOptions) (result *v1alpha1.ClusterDistributionPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterdistributionpoliciesResource, clusterDistributionPolicy), &v1alpha1.ClusterDistributionPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterDistributionPolicy), err
}

// Update takes the representation of a clusterDistributionPolicy and updates it. Returns the server's representation of the clusterDistributionPolicy, and an error, if there is any.
func (c *FakeClusterDistributionPolicies) Update(ctx context.Context, clusterDistributionPolicy *v1alpha1.ClusterDistributionPolicy, opts v1.UpdateOptions) (result *v1alpha1.ClusterDistributionPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterdistributionpoliciesResource, clusterDistributionPolicy), &v1alpha1.ClusterDistributionPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterDistributionPolicy), err
}

// Delete takes name of the clusterDistributionPolicy and deletes it. Returns an error if one occurs.
func (c *FakeClusterDistributionPolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(clusterdistributionpoliciesResource, name, opts), &v1alpha1.ClusterDistributionPolicy{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterDistributionPolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterdistributionpoliciesResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ClusterDistributionPolicyList{})
	return err
}

// Patch applies the patch and returns the patched clusterDistributionPolicy.
func (c *FakeClusterDistributionPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ClusterDistributionPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterdistributionpoliciesResource, name, pt, data, subresources...), &v1alpha1.ClusterDistributionPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterDistributionPolicy), err
}
