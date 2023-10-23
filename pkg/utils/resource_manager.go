package utils

import (
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	discoverylisterv1 "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
)

type ResourceManager struct {
	InformerFactory                kubeinformers.SharedInformerFactory
	KosmosInformerFactory          externalversions.SharedInformerFactory
	EndpointSliceInformer          cache.SharedInformer
	EndpointSliceLister            discoverylisterv1.EndpointSliceLister
	EndpointSliceInformerHasSynced bool
}

// NewResourceManager hold the informer manager of master cluster
func NewResourceManager(kubeClient kubernetes.Interface, kosmosClient versioned.Interface) *ResourceManager {
	informerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, DefaultInformerResyncPeriod)
	kosmosInformerFactory := externalversions.NewSharedInformerFactory(kosmosClient, DefaultInformerResyncPeriod)
	endpointSliceInformer := informerFactory.Discovery().V1().EndpointSlices()
	return &ResourceManager{
		InformerFactory:                informerFactory,
		KosmosInformerFactory:          kosmosInformerFactory,
		EndpointSliceInformer:          endpointSliceInformer.Informer(),
		EndpointSliceLister:            endpointSliceInformer.Lister(),
		EndpointSliceInformerHasSynced: endpointSliceInformer.Informer().HasSynced(),
	}
}
