// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/clusterlink/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterNodes implements ClusterNodeInterface
type FakeClusterNodes struct {
	Fake *FakeClusterlinkV1alpha1
}

var clusternodesResource = schema.GroupVersionResource{Group: "clusterlink.io", Version: "v1alpha1", Resource: "clusternodes"}

var clusternodesKind = schema.GroupVersionKind{Group: "clusterlink.io", Version: "v1alpha1", Kind: "ClusterNode"}

// Get takes name of the clusterNode, and returns the corresponding clusterNode object, and an error if there is any.
func (c *FakeClusterNodes) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ClusterNode, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusternodesResource, name), &v1alpha1.ClusterNode{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterNode), err
}

// List takes label and field selectors, and returns the list of ClusterNodes that match those selectors.
func (c *FakeClusterNodes) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ClusterNodeList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusternodesResource, clusternodesKind, opts), &v1alpha1.ClusterNodeList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ClusterNodeList{ListMeta: obj.(*v1alpha1.ClusterNodeList).ListMeta}
	for _, item := range obj.(*v1alpha1.ClusterNodeList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterNodes.
func (c *FakeClusterNodes) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusternodesResource, opts))
}

// Create takes the representation of a clusterNode and creates it.  Returns the server's representation of the clusterNode, and an error, if there is any.
func (c *FakeClusterNodes) Create(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.CreateOptions) (result *v1alpha1.ClusterNode, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusternodesResource, clusterNode), &v1alpha1.ClusterNode{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterNode), err
}

// Update takes the representation of a clusterNode and updates it. Returns the server's representation of the clusterNode, and an error, if there is any.
func (c *FakeClusterNodes) Update(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.UpdateOptions) (result *v1alpha1.ClusterNode, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusternodesResource, clusterNode), &v1alpha1.ClusterNode{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterNode), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeClusterNodes) UpdateStatus(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.UpdateOptions) (*v1alpha1.ClusterNode, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusternodesResource, "status", clusterNode), &v1alpha1.ClusterNode{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterNode), err
}

// Delete takes name of the clusterNode and deletes it. Returns an error if one occurs.
func (c *FakeClusterNodes) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(clusternodesResource, name, opts), &v1alpha1.ClusterNode{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterNodes) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusternodesResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ClusterNodeList{})
	return err
}

// Patch applies the patch and returns the patched clusterNode.
func (c *FakeClusterNodes) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ClusterNode, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusternodesResource, name, pt, data, subresources...), &v1alpha1.ClusterNode{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterNode), err
}