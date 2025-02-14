package delegate

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	clientrest "k8s.io/client-go/rest"

	store "github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/store"
)

type Proxy interface {
	Connect(ctx context.Context, request ProxyRequest) (http.Handler, error)
}

type Delegate interface {
	Proxy
	Order() int
	SupportRequest(request ProxyRequest) bool
}

var _ Proxy = (*delegateChain)(nil)

type Dependency struct {
	RestConfig *clientrest.Config
	RestMapper meta.RESTMapper

	MinRequestTimeout time.Duration
	Store             store.Store
}

type ProxyRequest struct {
	RequestInfo          *request.RequestInfo
	GroupVersionResource schema.GroupVersionResource
	ProxyPath            string

	Responder rest.Responder
	HTTPReq   *http.Request
}

type delegateChain struct {
	delegateList []Delegate
}

func NewDelegateChain(delegateList []Delegate) Proxy {
	sort.Slice(delegateList, func(i, j int) bool {
		return delegateList[i].Order() < delegateList[j].Order()
	})
	return &delegateChain{delegateList: delegateList}
}

// Connect implements Proxy.
func (d *delegateChain) Connect(ctx context.Context, request ProxyRequest) (http.Handler, error) {
	proxy, err := d.selectDelegate(request)
	if err != nil {
		return nil, err
	}

	return proxy.Connect(ctx, request)
}

func (d *delegateChain) selectDelegate(request ProxyRequest) (Delegate, error) {
	for _, delegate := range d.delegateList {
		if delegate.SupportRequest(request) {
			return delegate, nil
		}
	}

	return nil, fmt.Errorf("no plugin found for request: %v %v",
		request.RequestInfo.Verb, request.RequestInfo.Path)
}
