package informers

import (
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	klog2 "k8s.io/klog/v2"

	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	kosmosinformers "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
)

const DefaultResync = 3600 * time.Second

type InformerFactory interface {
	//K8sInformerFactory return the default InformerFactory
	K8sInformerFactory() informers.SharedInformerFactory

	//KosmosInformerFactory return the kosmos InformerFactory
	KosmosInformerFactory() kosmosinformers.SharedInformerFactory

	// SyncCache blocks until all register informers' caches were synced
	// or the stop channel gets closed.
	SyncCache(stopCh <-chan struct{}) error
}

type informerFactory struct {
	k8sClient    kubernetes.Interface
	kosmosClient kosmosversioned.Interface

	k8sInformerFactory    informers.SharedInformerFactory
	kosmosInformerFactory kosmosinformers.SharedInformerFactory

	k8sResources    map[schema.GroupVersion][]string
	kosmosResources map[schema.GroupVersion][]string
}

func NewInformerFactory(
	k8sClient kubernetes.Interface,
	kosmosClient kosmosversioned.Interface,
	k8sResources map[schema.GroupVersion][]string,
	kosmosResources map[schema.GroupVersion][]string) InformerFactory {
	if k8sClient == nil || kosmosClient == nil {
		klog2.Fatal("Leaf client is nil, exit os !")
	}
	k8sInformerFactory := informers.NewSharedInformerFactory(k8sClient, DefaultResync)
	kosmosInformerFactory := kosmosinformers.NewSharedInformerFactory(kosmosClient, DefaultResync)
	return &informerFactory{
		k8sClient:             k8sClient,
		kosmosClient:          kosmosClient,
		k8sInformerFactory:    k8sInformerFactory,
		kosmosInformerFactory: kosmosInformerFactory,
		k8sResources:          k8sResources,
		kosmosResources:       kosmosResources}
}

func (i *informerFactory) K8sInformerFactory() informers.SharedInformerFactory {
	return i.k8sInformerFactory
}

func (i *informerFactory) KosmosInformerFactory() kosmosinformers.SharedInformerFactory {
	return i.kosmosInformerFactory
}

// SyncCache blocks until all register informers' caches were synced
// or the stop channel gets closed.
func (i *informerFactory) SyncCache(stopCh <-chan struct{}) error {
	discoveryClient := i.k8sClient.Discovery()

	if i.k8sResources != nil && len(i.k8sResources) != 0 {
		registerFunc := func(resource schema.GroupVersionResource) (interface{}, error) {
			return i.k8sInformerFactory.ForResource(resource)
		}
		if err := registerCacheInSharedInformerFactory(discoveryClient, registerFunc, i.k8sResources); err != nil {
			return err
		}
	}
	i.k8sInformerFactory.Start(stopCh)
	i.k8sInformerFactory.WaitForCacheSync(stopCh)

	if i.kosmosResources != nil && len(i.kosmosResources) != 0 {
		registerFunc := func(resource schema.GroupVersionResource) (interface{}, error) {
			return i.kosmosInformerFactory.ForResource(resource)
		}
		if err := registerCacheInSharedInformerFactory(discoveryClient, registerFunc, i.kosmosResources); err != nil {
			return err
		}
	}

	i.kosmosInformerFactory.Start(stopCh)
	i.kosmosInformerFactory.WaitForCacheSync(stopCh)

	return nil
}

// registerCacheInSharedInformerFactory is for register gvr to informer factory
func registerCacheInSharedInformerFactory(discoveryClient discovery.DiscoveryInterface, registerFunc func(resource schema.GroupVersionResource) (interface{}, error), gvrs map[schema.GroupVersion][]string) error {
	for groupVersion, resourceNames := range gvrs {
		apiResourceList, err := discoveryClient.ServerResourcesForGroupVersion(groupVersion.String())
		if err != nil {
			klog2.Errorf("get %s ApiResource List error,", groupVersion.String(), err)
			return err
		}
		for _, resourceName := range resourceNames {
			if !apiResourceExists(apiResourceList.APIResources, resourceName) {
				klog2.Errorf("resource %s not exists in the cluster", resourceName)
			} else {
				groupVersionResource := groupVersion.WithResource(resourceName)
				if _, err = registerFunc(groupVersionResource); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// apiResourceExists judge the current gvr is exist
func apiResourceExists(apiResources []v1.APIResource, resourceName string) bool {
	for _, apiResource := range apiResources {
		if apiResource.Name == resourceName {
			return true
		}
	}
	return false
}
