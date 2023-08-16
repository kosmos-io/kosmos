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
)

// Store is the cache for resources from controlpanel
type Store interface {
	UpdateCache(resources map[schema.GroupVersionResource]struct{}) error
	GetResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error)
	Stop()

	Get(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error)
	List(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error)
	Watch(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error)
}

var _ Store = &Cache{}

// Cache caches resources
type Cache struct {
	lock       sync.RWMutex
	cache      map[schema.GroupVersionResource]*resourceCache
	restMapper meta.RESTMapper
	client     dynamic.Interface
}

func NewClusterCache(client dynamic.Interface, restMapper meta.RESTMapper) *Cache {
	// TODO add controller dynamic add clusterlink crd
	cache := &Cache{
		client:     client,
		restMapper: restMapper,
		cache:      map[schema.GroupVersionResource]*resourceCache{},
	}
	resources := map[schema.GroupVersionResource]struct{}{
		{Group: "clusterlink.io", Version: "v1alpha1", Resource: "clusters"}:     {},
		{Group: "clusterlink.io", Version: "v1alpha1", Resource: "clusternodes"}: {},
		{Group: "clusterlink.io", Version: "v1alpha1", Resource: "nodeconfigs"}:  {},
	}
	err := cache.UpdateCache(resources)
	if err != nil {
		panic(err)
	}
	return cache
}

func (c *Cache) UpdateCache(resources map[schema.GroupVersionResource]struct{}) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// remove non-exist resources
	for resource := range c.cache {
		if _, exist := resources[resource]; !exist {
			klog.Infof("Remove cache for %s", resource.String())
			c.cache[resource].stop()
			delete(c.cache, resource)
		}
	}

	// add resource cache
	for resource := range resources {
		_, exist := c.cache[resource]
		if !exist {
			kind, err := c.restMapper.KindFor(resource)
			if err != nil {
				return err
			}
			mapping, err := c.restMapper.RESTMapping(kind.GroupKind(), kind.Version)
			if err != nil {
				return err
			}
			namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace

			klog.Infof("Add cache for %s", resource.String())
			cache, err := newResourceCache(resource, kind, namespaced, c.clientForResourceFunc(resource))
			if err != nil {
				return err
			}
			c.cache[resource] = cache
		}
	}
	return nil
}

func (c *Cache) Stop() {
	c.lock.RLock()
	defer c.lock.RUnlock()

	for _, cache := range c.cache {
		cache.stop()
	}
}

func (c *Cache) GetResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
	return nil, "", nil
}

func (c *Cache) Get(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error) {
	rc := c.cacheForResource(gvr)
	if rc == nil {
		return nil, errors.New(fmt.Sprintf("can not find gvr %v", gvr))
	}
	obj, err := rc.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}

	cloneObj := obj.DeepCopyObject()
	return cloneObj, err
}

func (c *Cache) Update(ctx context.Context, gvr schema.GroupVersionResource, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	rc := c.cacheForResource(gvr)
	if rc == nil {
		// TODO
		return nil, false, errors.New("can not find gvr")
	}
	return rc.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

func (c *Cache) List(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error) {
	rc := c.cacheForResource(gvr)
	if rc == nil {
		// TODO
		return nil, errors.New(fmt.Sprintf("can not find target gvr %v", gvr))
	}
	list, err := rc.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Cache) Watch(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error) {
	rc := c.cacheForResource(gvr)
	if rc == nil {
		return nil, errors.New(fmt.Sprintf("can not find target gvr %v", gvr))
	}
	w, err := rc.Watch(ctx, options)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (c *Cache) clientForResourceFunc(resource schema.GroupVersionResource) func() (dynamic.NamespaceableResourceInterface, error) {
	return func() (dynamic.NamespaceableResourceInterface, error) {
		return c.client.Resource(resource), nil
	}
}

func (c *Cache) cacheForResource(gvr schema.GroupVersionResource) *resourceCache {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.cache[gvr]
}
