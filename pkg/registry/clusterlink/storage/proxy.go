package storage

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"k8s.io/apimachinery/pkg/runtime"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	k8srest "k8s.io/apiserver/pkg/registry/rest"

	proxyv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/proxy/v1alpha1"
	clusterlinkproxy "github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/controller"
)

var supportMethods = []string{"GET", "DELETE", "POST", "PUT", "PATCH", "HEAD", "OPTIONS"}

type ProxyREST struct {
	ctl *clusterlinkproxy.ResourceCacheController
}

var _ k8srest.Scoper = &ProxyREST{}
var _ k8srest.Storage = &ProxyREST{}
var _ k8srest.Connecter = &ProxyREST{}

func NewProxyREST(ctl *clusterlinkproxy.ResourceCacheController) *ProxyREST {
	return &ProxyREST{
		ctl: ctl,
	}
}

func (r *ProxyREST) New() runtime.Object {
	return &proxyv1alpha1.Proxying{}
}

// NamespaceScoped returns false because Storage is not namespaced.
func (r *ProxyREST) NamespaceScoped() bool {
	return false
}

// ConnectMethods returns the list of HTTP methods handled by Connect.
func (r *ProxyREST) ConnectMethods() []string {
	return supportMethods
}

// NewConnectOptions returns an empty options object that will be used to pass options to the Connect method.
func (r *ProxyREST) NewConnectOptions() (runtime.Object, bool, string) {
	return nil, true, ""
}

// Connect returns a handler for proxy.
func (r *ProxyREST) Connect(ctx context.Context, _ string, _ runtime.Object, responder k8srest.Responder) (http.Handler, error) {
	info, ok := genericrequest.RequestInfoFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no RequestInfo found in the context")
	}

	if len(info.Parts) < 2 {
		return nil, fmt.Errorf("invalid requestInfo parts: %v", info.Parts)
	}

	proxyPath := "/" + path.Join(info.Parts[3:]...)
	return r.ctl.Connect(ctx, proxyPath, responder)
}

// Destroy cleans up its resources on shutdown.
func (r *ProxyREST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}

func (r *ProxyREST) GetSingularName() string {
	return "proxying"
}
