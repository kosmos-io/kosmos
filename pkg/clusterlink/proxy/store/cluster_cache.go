package store

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

// Store is the cache for resources from controlpanel
type Store interface {
	UpdateCache(resources map[schema.GroupVersionResource]*utils.MultiNamespace) error
	HasResource(resource schema.GroupVersionResource) bool
	GetResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error)
	Stop()

	Get(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error)
	List(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error)
	Watch(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error)
}

var _ Store = &ClusterCache{}

// ClusterCache caches resources
type ClusterCache struct {
	lock       sync.RWMutex
	cache      map[schema.GroupVersionResource]*resourceCache
	restMapper meta.RESTMapper
	client     dynamic.Interface
}

// ReadinessCheck implements Store.

func NewClusterCache(client dynamic.Interface, restMapper meta.RESTMapper) *ClusterCache {
	cache := &ClusterCache{
		client:     client,
		restMapper: restMapper,
		cache:      map[schema.GroupVersionResource]*resourceCache{},
	}
	return cache
}

func (c *ClusterCache) UpdateCache(resources map[schema.GroupVersionResource]*utils.MultiNamespace) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// remove non-exist resources and namespaces changed resource
	for gvr, rc := range c.cache {
		if namespaces, exist := resources[gvr]; !exist || !rc.namespaces.Equal(namespaces) {
			klog.Infof("Remove cache for %s", gvr.String())
			c.cache[gvr].stop()
			delete(c.cache, gvr)
		}
	}

	// add resource cache
	for gvr, namespaces := range resources {
		if _, exist := c.cache[gvr]; !exist {
			kind, err := c.restMapper.KindFor(gvr)
			if err != nil {
				return err
			}
			mapping, err := c.restMapper.RESTMapping(kind.GroupKind(), kind.Version)
			if err != nil {
				return err
			}
			namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace

			klog.Infof("Add cache for %s", gvr.String())
			cache, err := newResourceCache(gvr, kind, namespaced, namespaces, c.clientForResourceFunc(gvr))
			if err != nil {
				return err
			}
			c.cache[gvr] = cache
		}
	}
	return nil
}

func (c *ClusterCache) Stop() {
	c.lock.RLock()
	defer c.lock.RUnlock()

	for _, cache := range c.cache {
		cache.stop()
	}
}

func (c *ClusterCache) GetResourceFromCache(_ context.Context, _ schema.GroupVersionResource, _, _ string) (runtime.Object, string, error) {
	return nil, "", nil
}

func (c *ClusterCache) Get(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error) {
	rc := c.cacheForResource(gvr)
	if rc == nil {
		return nil, fmt.Errorf("can not find gvr %v", gvr)
	}
	obj, err := rc.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}

	cloneObj := obj.DeepCopyObject()
	return cloneObj, err
}

func (c *ClusterCache) Update(ctx context.Context, gvr schema.GroupVersionResource, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	rc := c.cacheForResource(gvr)
	if rc == nil {
		// TODO
		return nil, false, errors.New("can not find gvr")
	}
	return rc.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

func (c *ClusterCache) List(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error) {
	rc := c.cacheForResource(gvr)
	if rc == nil {
		return nil, fmt.Errorf("can not find target gvr %v", gvr)
	}
	if options.ResourceVersion == "" {
		options.ResourceVersion = "0"
	}
	list, err := rc.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (c *ClusterCache) Watch(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error) {
	rc := c.cacheForResource(gvr)
	if rc == nil {
		return nil, fmt.Errorf("can not find target gvr %v", gvr)
	}
	w, err := rc.Watch(ctx, options)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (c *ClusterCache) HasResource(resource schema.GroupVersionResource) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, ok := c.cache[resource]
	return ok
}

// get the client for the resource
func (c *ClusterCache) clientForResourceFunc(resource schema.GroupVersionResource) func() (dynamic.NamespaceableResourceInterface, error) {
	return func() (dynamic.NamespaceableResourceInterface, error) {
		if c.client == nil {
			return nil, errors.New("client is nil")
		}
		return c.client.Resource(resource), nil
	}
}

// get the cache for the resource
func (c *ClusterCache) cacheForResource(gvr schema.GroupVersionResource) *resourceCache {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.cache[gvr]
}
