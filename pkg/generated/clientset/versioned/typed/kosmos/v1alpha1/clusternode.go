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

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	scheme "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ClusterNodesGetter has a method to return a ClusterNodeInterface.
// A group's client should implement this interface.
type ClusterNodesGetter interface {
	ClusterNodes() ClusterNodeInterface
}

// ClusterNodeInterface has methods to work with ClusterNode resources.
type ClusterNodeInterface interface {
	Create(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.CreateOptions) (*v1alpha1.ClusterNode, error)
	Update(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.UpdateOptions) (*v1alpha1.ClusterNode, error)
	UpdateStatus(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.UpdateOptions) (*v1alpha1.ClusterNode, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ClusterNode, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ClusterNodeList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ClusterNode, err error)
	ClusterNodeExpansion
}

// clusterNodes implements ClusterNodeInterface
type clusterNodes struct {
	client rest.Interface
}

// newClusterNodes returns a ClusterNodes
func newClusterNodes(c *KosmosV1alpha1Client) *clusterNodes {
	return &clusterNodes{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterNode, and returns the corresponding clusterNode object, and an error if there is any.
func (c *clusterNodes) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ClusterNode, err error) {
	result = &v1alpha1.ClusterNode{}
	err = c.client.Get().
		Resource("clusternodes").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterNodes that match those selectors.
func (c *clusterNodes) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ClusterNodeList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ClusterNodeList{}
	err = c.client.Get().
		Resource("clusternodes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterNodes.
func (c *clusterNodes) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("clusternodes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a clusterNode and creates it.  Returns the server's representation of the clusterNode, and an error, if there is any.
func (c *clusterNodes) Create(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.CreateOptions) (result *v1alpha1.ClusterNode, err error) {
	result = &v1alpha1.ClusterNode{}
	err = c.client.Post().
		Resource("clusternodes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(clusterNode).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a clusterNode and updates it. Returns the server's representation of the clusterNode, and an error, if there is any.
func (c *clusterNodes) Update(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.UpdateOptions) (result *v1alpha1.ClusterNode, err error) {
	result = &v1alpha1.ClusterNode{}
	err = c.client.Put().
		Resource("clusternodes").
		Name(clusterNode.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(clusterNode).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *clusterNodes) UpdateStatus(ctx context.Context, clusterNode *v1alpha1.ClusterNode, opts v1.UpdateOptions) (result *v1alpha1.ClusterNode, err error) {
	result = &v1alpha1.ClusterNode{}
	err = c.client.Put().
		Resource("clusternodes").
		Name(clusterNode.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(clusterNode).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the clusterNode and deletes it. Returns an error if one occurs.
func (c *clusterNodes) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusternodes").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterNodes) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("clusternodes").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched clusterNode.
func (c *clusterNodes) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ClusterNode, err error) {
	result = &v1alpha1.ClusterNode{}
	err = c.client.Patch(pt).
		Resource("clusternodes").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
