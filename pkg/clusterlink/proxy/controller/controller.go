package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/metrics"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	clientrest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/delegate"
	apiserverdelegate "github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/delegate/apiserver"
	cachedelegate "github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/delegate/cache"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/store"
	informerfactory "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	lister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

const (
	maxRetries        = 15
	minRequestTimeout = 15
)

type ResourceCacheController struct {
	restMapper           meta.RESTMapper
	negotiatedSerializer runtime.NegotiatedSerializer

	cacheSynced          cache.InformerSynced
	resourceCacheLister  lister.ResourceCacheLister
	queue                workqueue.RateLimitingInterface
	enqueueResourceCache func(obj *v1alpha1.ResourceCache)
	syncHandler          func(key string) error
	store                store.Store
	delegate             delegate.Proxy
}

type NewControllerOption struct {
	RestConfig    *clientrest.Config
	DynamicClient dynamic.Interface
	RestMapper    meta.RESTMapper
	KosmosFactory informerfactory.SharedInformerFactory
}

func NewResourceCacheController(option NewControllerOption) (*ResourceCacheController, error) {
	store := store.NewClusterCache(option.DynamicClient, option.RestMapper)

	rc := &ResourceCacheController{
		restMapper:           option.RestMapper,
		negotiatedSerializer: scheme.Codecs.WithoutConversion(),
		store:                store,
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "resourceCache"),
		cacheSynced:          option.KosmosFactory.Kosmos().V1alpha1().ResourceCaches().Informer().HasSynced,
		resourceCacheLister:  option.KosmosFactory.Kosmos().V1alpha1().ResourceCaches().Lister(),
	}

	resourceEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r := obj.(*v1alpha1.ResourceCache)
			klog.V(4).InfoS("Adding ResourceCache", "resourceCache", klog.KObj(r))
			rc.eventHandlerFunc(r)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldR := old.(*v1alpha1.ResourceCache)
			klog.V(4).InfoS("Updating ResourceCache", "resourceCache", klog.KObj(oldR))
			curR := cur.(*v1alpha1.ResourceCache)
			rc.eventHandlerFunc(curR)
		},
		DeleteFunc: func(obj interface{}) {
			r := obj.(*v1alpha1.ResourceCache)
			klog.V(4).InfoS("Deleting ResourceCache", "resourceCache", klog.KObj(r))
			rc.eventHandlerFunc(r)
		},
	}

	_, err := option.KosmosFactory.Kosmos().V1alpha1().ResourceCaches().Informer().AddEventHandler(resourceEventHandler)
	if err != nil {
		klog.Errorf("Failed to add handler for Clusters: %v", err)
		return nil, err
	}

	// set delegate for proxy request
	delegates, err := newDelegates(option, store)
	if err != nil {
		return nil, err
	}
	rc.delegate = delegate.NewDelegateChain(delegates)

	rc.enqueueResourceCache = rc.enqueue
	rc.syncHandler = rc.syncResourceCache
	return rc, nil
}

// newDelegates the delegates for proxy request: cache->apiserver
func newDelegates(option NewControllerOption, store store.Store) ([]delegate.Delegate, error) {
	delegateDependency := delegate.Dependency{
		RestConfig:        option.RestConfig,
		RestMapper:        option.RestMapper,
		Store:             store,
		MinRequestTimeout: minRequestTimeout * time.Second,
	}
	allDelegates := make([]delegate.Delegate, 0, 2)
	allDelegates = append(allDelegates, cachedelegate.New(delegateDependency))
	apiserverdelegate, err := apiserverdelegate.New(delegateDependency)
	if err != nil {
		return allDelegates, err
	}
	allDelegates = append(allDelegates, apiserverdelegate)
	return allDelegates, nil
}

func (rc *ResourceCacheController) eventHandlerFunc(obj *v1alpha1.ResourceCache) {
	rc.enqueueResourceCache(obj)
}

func (rc *ResourceCacheController) enqueue(obj *v1alpha1.ResourceCache) {
	name := obj.GetObjectMeta().GetName()
	rc.queue.Add(name)
}

// syncResourceCache will sync the resourceCache CR
// First list all resources, and then compare the previous cache to find out which are deleted
// and which are newly added
func (rc *ResourceCacheController) syncResourceCache(_ string) error {
	// list all resourceCache CR
	resourcesCaches, err := rc.resourceCacheLister.List(labels.Everything())
	if err != nil {
		return err
	}
	// Define a map deduplication gvr
	cachedResources := make(map[schema.GroupVersionResource]*utils.MultiNamespace)

	for _, resourceCache := range resourcesCaches {
		for _, selector := range resourceCache.Spec.ResourceCacheSelectors {
			gvr, err := rc.getGroupVersionResource(rc.restMapper, schema.FromAPIVersionAndKind(selector.APIVersion, selector.Kind))
			if err != nil {
				klog.Errorf("Failed to get gvr: %v", err)
				continue
			}
			nsSelector, ok := cachedResources[gvr]
			if !ok {
				nsSelector = utils.NewMultiNamespace()
				cachedResources[gvr] = nsSelector
			}
			nsSelector.Add(selector.Namespace...)
			cachedResources[gvr] = nsSelector
		}
	}

	return rc.store.UpdateCache(cachedResources)
}

func (rc *ResourceCacheController) getGroupVersionResource(restMapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return restMapping.Resource, nil
}

func (rc *ResourceCacheController) Run(stopCh <-chan struct{}, workers int) {
	defer utilruntime.HandleCrash()
	defer rc.queue.ShutDown()

	klog.InfoS("Starting controller", "controller", "resourceCache")
	defer klog.InfoS("Shutting down controller", "controller", "resourceCache")

	klog.Info("Waiting for caches to sync for resourceCache controller")
	if !cache.WaitForCacheSync(stopCh, rc.cacheSynced) {
		utilruntime.HandleError(fmt.Errorf("Unable to sync caches for resourceCache controller"))
		return
	}
	klog.Infof("Caches are synced for resourceCachecontroller")

	for i := 0; i < workers; i++ {
		go wait.Until(rc.worker, time.Second, stopCh)
	}
	<-stopCh
}

func (rc ResourceCacheController) worker() {
	for rc.processNextWorkItem() {
	}
}

func (rc *ResourceCacheController) processNextWorkItem() bool {
	key, quit := rc.queue.Get()
	if quit {
		return false
	}
	defer rc.queue.Done(key)

	err := rc.syncHandler(key.(string))
	rc.handleErr(err, key)

	return true
}

func (rc *ResourceCacheController) handleErr(err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
		rc.queue.Forget(key)
		return
	}

	ns, name, keyErr := cache.SplitMetaNamespaceKey(key.(string))
	if keyErr != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
	}

	if rc.queue.NumRequeues(key) < maxRetries {
		klog.V(2).InfoS("Error syncing resourceCache", "resourceCache", klog.KRef(ns, name), "err", err)
		rc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(2).InfoS("Dropping resourceCache out of the queue", "resourceCache", klog.KRef(ns, name), "err", err)
	rc.queue.Forget(key)
}

// Connect handler for proxy request use delegate chain
func (rc *ResourceCacheController) Connect(ctx context.Context, proxyPath string, responder rest.Responder) (http.Handler, error) {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		newReq := req.Clone(req.Context())
		newReq.URL.Path = proxyPath
		requestInfo := lifted.NewRequestInfo(newReq)

		newCtx := request.WithRequestInfo(ctx, requestInfo)
		newCtx = request.WithNamespace(newCtx, requestInfo.Namespace)
		newReq = newReq.WithContext(newCtx)

		gvr := schema.GroupVersionResource{
			Group:    requestInfo.APIGroup,
			Version:  requestInfo.APIVersion,
			Resource: requestInfo.Resource,
		}

		h, err := rc.delegate.Connect(newCtx, delegate.ProxyRequest{
			RequestInfo:          requestInfo,
			GroupVersionResource: gvr,
			ProxyPath:            proxyPath,
			Responder:            responder,
			HTTPReq:              newReq,
		})

		if err != nil {
			h = &errorHTTPHandler{
				requestInfo:          requestInfo,
				err:                  err,
				negotiatedSerializer: rc.negotiatedSerializer,
			}
		}

		h = metrics.InstrumentHandlerFunc(requestInfo.Verb, requestInfo.APIGroup, requestInfo.APIVersion, requestInfo.Resource, requestInfo.Subresource,
			"", utils.KosmosClusrerLinkRroxyComponentName, false, "", h.ServeHTTP)
		h.ServeHTTP(rw, newReq)
	}), nil
}

type errorHTTPHandler struct {
	requestInfo          *request.RequestInfo
	err                  error
	negotiatedSerializer runtime.NegotiatedSerializer
}

func (handler *errorHTTPHandler) ServeHTTP(delegate http.ResponseWriter, req *http.Request) {
	// Write error into delegate ResponseWriter, wrapped in metrics.InstrumentHandlerFunc, so metrics can record this error.
	gv := schema.GroupVersion{
		Group:   handler.requestInfo.APIGroup,
		Version: handler.requestInfo.Verb,
	}
	responsewriters.ErrorNegotiated(handler.err, handler.negotiatedSerializer, gv, delegate, req)
}
