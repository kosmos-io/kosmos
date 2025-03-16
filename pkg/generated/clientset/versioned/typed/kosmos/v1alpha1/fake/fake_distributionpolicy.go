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

// FakeDistributionPolicies implements DistributionPolicyInterface
type FakeDistributionPolicies struct {
	Fake *FakeKosmosV1alpha1
	ns   string
}

var distributionpoliciesResource = schema.GroupVersionResource{Group: "kosmos.io", Version: "v1alpha1", Resource: "distributionpolicies"}

var distributionpoliciesKind = schema.GroupVersionKind{Group: "kosmos.io", Version: "v1alpha1", Kind: "DistributionPolicy"}

// Get takes name of the distributionPolicy, and returns the corresponding distributionPolicy object, and an error if there is any.
func (c *FakeDistributionPolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.DistributionPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(distributionpoliciesResource, c.ns, name), &v1alpha1.DistributionPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistributionPolicy), err
}

// List takes label and field selectors, and returns the list of DistributionPolicies that match those selectors.
func (c *FakeDistributionPolicies) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.DistributionPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(distributionpoliciesResource, distributionpoliciesKind, c.ns, opts), &v1alpha1.DistributionPolicyList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.DistributionPolicyList{ListMeta: obj.(*v1alpha1.DistributionPolicyList).ListMeta}
	for _, item := range obj.(*v1alpha1.DistributionPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested distributionPolicies.
func (c *FakeDistributionPolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(distributionpoliciesResource, c.ns, opts))

}

// Create takes the representation of a distributionPolicy and creates it.  Returns the server's representation of the distributionPolicy, and an error, if there is any.
func (c *FakeDistributionPolicies) Create(ctx context.Context, distributionPolicy *v1alpha1.DistributionPolicy, opts v1.CreateOptions) (result *v1alpha1.DistributionPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(distributionpoliciesResource, c.ns, distributionPolicy), &v1alpha1.DistributionPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistributionPolicy), err
}

// Update takes the representation of a distributionPolicy and updates it. Returns the server's representation of the distributionPolicy, and an error, if there is any.
func (c *FakeDistributionPolicies) Update(ctx context.Context, distributionPolicy *v1alpha1.DistributionPolicy, opts v1.UpdateOptions) (result *v1alpha1.DistributionPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(distributionpoliciesResource, c.ns, distributionPolicy), &v1alpha1.DistributionPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistributionPolicy), err
}

// Delete takes name of the distributionPolicy and deletes it. Returns an error if one occurs.
func (c *FakeDistributionPolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(distributionpoliciesResource, c.ns, name, opts), &v1alpha1.DistributionPolicy{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDistributionPolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(distributionpoliciesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.DistributionPolicyList{})
	return err
}

// Patch applies the patch and returns the patched distributionPolicy.
func (c *FakeDistributionPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.DistributionPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(distributionpoliciesResource, c.ns, name, pt, data, subresources...), &v1alpha1.DistributionPolicy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistributionPolicy), err
}
