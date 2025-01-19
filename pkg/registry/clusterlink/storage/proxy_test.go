package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"

	proxyv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/proxy/v1alpha1"
	clusterlinkproxy "github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/controller"
)

func TestNewProxyREST(t *testing.T) {
	ctl := clusterlinkproxy.ResourceCacheController{}
	r := NewProxyREST(&ctl)
	assert.NotNil(t, r)
}

func TestProxyREST_New(t *testing.T) {
	ctl := clusterlinkproxy.ResourceCacheController{}
	r := NewProxyREST(&ctl)
	obj := r.New()
	assert.NotNil(t, obj)
	_, ok := obj.(*proxyv1alpha1.Proxying)
	assert.True(t, ok)
}

func TestNewProxyREST_NamespaceScoped(t *testing.T) {
	ctl := &clusterlinkproxy.ResourceCacheController{}
	r := NewProxyREST(ctl)
	assert.False(t, r.NamespaceScoped())
}

func TestNewProxyREST_ConnectMethods(t *testing.T) {
	ctl := &clusterlinkproxy.ResourceCacheController{}
	r := NewProxyREST(ctl)
	methods := r.ConnectMethods()
	assert.Equal(t, supportMethods, methods)
}

func TestNewProxyREST_NewConnectOptions(t *testing.T) {
	ctl := &clusterlinkproxy.ResourceCacheController{}
	r := NewProxyREST(ctl)
	obj, ok, s := r.NewConnectOptions()
	assert.Nil(t, obj)
	assert.True(t, ok)
	assert.Equal(t, "", s)
}

func TestProxyREST_Destroy(_ *testing.T) {
	ctl := clusterlinkproxy.ResourceCacheController{}
	r := NewProxyREST(&ctl)
	r.Destroy()
}

func TestProxyREST_Connect(t *testing.T) {
	ctl := clusterlinkproxy.ResourceCacheController{}
	r := NewProxyREST(&ctl)

	t.Run("Test missing RequestInfo in context", func(t *testing.T) {
		ctx := context.Background()
		_, err := r.Connect(ctx, "", nil, nil)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "no RequestInfo found in the context")
	})

	t.Run("Test invalid RequestInfo parts", func(t *testing.T) {
		ctx := genericrequest.WithRequestInfo(context.Background(), &genericrequest.RequestInfo{
			Parts: []string{"proxying"},
		})
		_, err := r.Connect(ctx, "", nil, nil)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "invalid requestInfo parts: [proxying]")
	})
}
