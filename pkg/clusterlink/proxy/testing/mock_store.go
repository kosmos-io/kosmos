package testing

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/store"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type MockStore struct {
	GetResourceFromCacheFunc func(ctx context.Context, gvr schema.GroupVersionResource, namespace string, name string) (runtime.Object, string, error)
	HasResourceFunc          func(resource schema.GroupVersionResource) bool
	ListFunc                 func(ctx context.Context, gvr schema.GroupVersionResource, options *internalversion.ListOptions) (runtime.Object, error)
	UpdateCacheFunc          func(resources map[schema.GroupVersionResource]*utils.MultiNamespace) error
	StopFunc                 func()
	WatchFunc                func(ctx context.Context, gvr schema.GroupVersionResource, options *internalversion.ListOptions) (watch.Interface, error)
	GetFunc                  func(ctx context.Context, gvr schema.GroupVersionResource, name string, options *v1.GetOptions) (runtime.Object, error)
}

// Get implements store.Store.
func (m *MockStore) Get(ctx context.Context, gvr schema.GroupVersionResource, name string, options *v1.GetOptions) (runtime.Object, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, gvr, name, options)
	}
	panic("unimplemented")
}

// GetResourceFromCache implements store.Store.
func (m *MockStore) GetResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace string, name string) (runtime.Object, string, error) {
	if m.GetResourceFromCacheFunc != nil {
		return m.GetResourceFromCacheFunc(ctx, gvr, namespace, name)
	}
	panic("unimplemented")
}

// HasResource implements store.Store.
func (m *MockStore) HasResource(resource schema.GroupVersionResource) bool {
	if m.HasResourceFunc != nil {
		return m.HasResourceFunc(resource)
	}
	panic("unimplemented")
}

// List implements store.Store.
func (m *MockStore) List(ctx context.Context, gvr schema.GroupVersionResource, options *internalversion.ListOptions) (runtime.Object, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, gvr, options)
	}
	panic("unimplemented")
}

// Stop implements store.Store.
func (m *MockStore) Stop() {
	if m.StopFunc != nil {
		m.StopFunc()
	}
	panic("unimplemented")
}

// UpdateCache implements store.Store.
func (m *MockStore) UpdateCache(resources map[schema.GroupVersionResource]*utils.MultiNamespace) error {
	if m.UpdateCacheFunc != nil {
		return m.UpdateCacheFunc(resources)
	}
	panic("unimplemented")
}

// Watch implements store.Store.
func (m *MockStore) Watch(ctx context.Context, gvr schema.GroupVersionResource, options *internalversion.ListOptions) (watch.Interface, error) {
	if m.WatchFunc != nil {
		return m.WatchFunc(ctx, gvr, options)
	}
	panic("unimplemented")
}

var _ store.Store = &MockStore{}
