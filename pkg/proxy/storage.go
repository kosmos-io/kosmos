package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/handlers"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	k8srest "k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clientrest "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/proxy/store"
	"github.com/kosmos.io/clusterlink/pkg/utils/lifted"
)

var supportMethods = []string{"GET", "DELETE", "POST", "PUT", "PATCH", "HEAD", "OPTIONS"}

type REST struct {
	store      store.Store
	config     *rest.Config
	restMapper meta.RESTMapper
}

func NewREST(config *rest.Config, client dynamic.Interface) *REST {

	restMapper, err := apiutil.NewDynamicRESTMapper(config)
	if err != nil {
		panic(err)
	}
	store := store.NewClusterCache(client, restMapper)
	return &REST{
		store:      store,
		config:     config,
		restMapper: restMapper,
	}
}

// New return empty Proxy object.
func (rest *REST) New() runtime.Object {
	return &v1alpha1.Proxy{}
}

// NamespaceScoped returns false because Storage is not namespaced.
func (rest *REST) NamespaceScoped() bool {
	return false
}

// ConnectMethods returns the list of HTTP methods handled by Connect.
func (rest *REST) ConnectMethods() []string {
	return supportMethods
}

// NewConnectOptions returns an empty options object that will be used to pass options to the Connect method.
func (rest *REST) NewConnectOptions() (runtime.Object, bool, string) {
	return nil, true, ""
}

// Connect returns a handler for proxy.
func (rest *REST) Connect(ctx context.Context, _ string, _ runtime.Object, responder k8srest.Responder) (http.Handler, error) {
	info, ok := genericrequest.RequestInfoFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no RequestInfo found in the context")
	}

	proxyHandler := rest.createProxyHandler(responder)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		paths := []string{info.APIPrefix, info.APIGroup, info.APIVersion, info.Resource}
		serverPrefix := "/" + path.Join(paths...)
		http.StripPrefix(serverPrefix, rest.createHandler(ctx, 300*time.Second, proxyHandler)).ServeHTTP(w, req)
	}), nil
}

func (rest *REST) createHandler(ctx context.Context, minRequestTimeout time.Duration, proxyHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		newReq := req.Clone(req.Context())
		requestInfo := lifted.NewRequestInfo(req)

		newCtx := request.WithRequestInfo(ctx, requestInfo)
		newCtx = request.WithNamespace(newCtx, requestInfo.Namespace)
		newReq = newReq.WithContext(newCtx)

		gvr := schema.GroupVersionResource{
			Group:    requestInfo.APIGroup,
			Version:  requestInfo.APIVersion,
			Resource: requestInfo.Resource,
		}

		if gvr.Group == "" && gvr.Resource == "" {
			proxyHandler.ServeHTTP(w, req)
			return
		}

		r := &rester{
			store:          rest.store,
			gvr:            gvr,
			tableConvertor: k8srest.NewDefaultTableConvertor(gvr.GroupResource()),
		}
		gvk, err := rest.restMapper.KindFor(gvr)
		if err != nil {
			responsewriters.ErrorNegotiated(
				apierrors.NewInternalError(err),
				Codecs, schema.GroupVersion{}, w, req,
			)
			return
		}

		mapping, err := rest.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		clusterScoped := mapping.Scope.Name() == meta.RESTScopeNameRoot
		scope := &handlers.RequestScope{
			Kind:     gvk,
			Resource: gvr,
			Namer: &handlers.ContextBasedNaming{
				Namer:         meta.NewAccessor(),
				ClusterScoped: clusterScoped,
			},
			Serializer:       scheme.Codecs.WithoutConversion(),
			Convertor:        runtime.NewScheme(),
			Subresource:      requestInfo.Subresource,
			MetaGroupVersion: metav1.SchemeGroupVersion,
		}
		var h http.Handler
		switch requestInfo.Verb {
		case "get":
			h = handlers.GetResource(r, scope)
		case "list", "watch":
			h = handlers.ListResource(r, r, scope, false, minRequestTimeout)
		case "patch", "update", "create", "delete":
			h = proxyHandler
		default:
			responsewriters.ErrorNegotiated(
				apierrors.NewMethodNotSupported(gvr.GroupResource(), requestInfo.Verb),
				Codecs, gvr.GroupVersion(), w, req,
			)
		}
		if h != nil {
			h.ServeHTTP(w, newReq)
		}
	})
}

func (rest *REST) createProxyHandler(responder k8srest.Responder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		kubernetes, err := url.Parse(rest.config.Host)
		if err != nil {
			responsewriters.ErrorNegotiated(
				apierrors.NewInternalError(err),
				Codecs, schema.GroupVersion{}, w, req,
			)
			return
		}
		s := *req.URL
		s.Host = kubernetes.Host
		s.Scheme = kubernetes.Scheme
		req.Header.Del("Authorization")
		defaultTransport, err := clientrest.TransportFor(rest.config)
		httpProxy := proxy.NewUpgradeAwareHandler(&s, defaultTransport, true, false, proxy.NewErrorResponder(responder))
		httpProxy.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(defaultTransport, defaultTransport)
		httpProxy.ServeHTTP(w, req)
	})
}

// Destroy cleans up its resources on shutdown.
func (rest *REST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}

type rester struct {
	store          store.Store
	gvr            schema.GroupVersionResource
	tableConvertor k8srest.TableConvertor
}

var _ k8srest.Getter = &rester{}
var _ k8srest.Lister = &rester{}
var _ k8srest.Watcher = &rester{}

// Get implements rest.Getter interface
func (r *rester) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, r.gvr, name, options)
}

// Watch implements rest.Watcher interface
func (r *rester) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return r.store.Watch(ctx, r.gvr, options)
}

// List implements rest.Lister interface
func (r *rester) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return r.store.List(ctx, r.gvr, options)
}

// NewList implements rest.Lister interface
func (r *rester) NewList() runtime.Object {
	return &unstructured.UnstructuredList{}
}

// ConvertToTable implements rest.Lister interface
func (r *rester) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return r.tableConvertor.ConvertToTable(ctx, object, tableOptions)
}

func (r *rester) New() runtime.Object {
	return &unstructured.Unstructured{}
}
