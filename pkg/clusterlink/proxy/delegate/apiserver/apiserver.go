package delegate

import (
	"context"
	"net/http"
	"net/url"
	"path"

	proxyutil "k8s.io/apimachinery/pkg/util/proxy"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/delegate"
)

const (
	order = 3000
)

type Apiserver struct {
	proxyLocation  *url.URL
	proxyTransport http.RoundTripper
}

var _ delegate.Delegate = &Apiserver{}

func New(dep delegate.Dependency) (delegate.Delegate, error) {
	location, err := url.Parse(dep.RestConfig.Host)
	if err != nil {
		return nil, err
	}

	transport, err := restclient.TransportFor(dep.RestConfig)
	if err != nil {
		return nil, err
	}

	return &Apiserver{
		proxyLocation:  location,
		proxyTransport: transport,
	}, nil
}

// Connect implements Delegate.
func (a *Apiserver) Connect(_ context.Context, request delegate.ProxyRequest) (http.Handler, error) {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		klog.Infof("proxy to apiserver path: %s", request.ProxyPath)
		location, transport := a.resourceLocation()
		location.Path = path.Join(location.Path, request.ProxyPath)
		location.RawQuery = req.URL.RawQuery

		handler := proxyutil.NewUpgradeAwareHandler(location, transport, true, false, proxyutil.NewErrorResponder(request.Responder))
		handler.ServeHTTP(rw, req)
	}), nil
}

// Order implements Delegate.
func (a *Apiserver) Order() int {
	return order
}

// SupportRequest implements Delegate.
func (a *Apiserver) SupportRequest(_ delegate.ProxyRequest) bool {
	return true
}

func (a *Apiserver) resourceLocation() (*url.URL, http.RoundTripper) {
	location := *a.proxyLocation
	return &location, a.proxyTransport
}
