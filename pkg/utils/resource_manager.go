package utils

import (
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoverylisterv1 "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
)

type ResourceManager struct {
	InformerFactory       kubeinformers.SharedInformerFactory
	KosmosInformerFactory externalversions.SharedInformerFactory
	EndpointSliceInformer cache.SharedInformer
	EndpointSliceLister   discoverylisterv1.EndpointSliceLister
	ServiceInformer       cache.SharedInformer
	ServiceLister         corev1listers.ServiceLister
}

// NewResourceManager hold the informer manager of master cluster
func NewResourceManager(kubeClient kubernetes.Interface, kosmosClient versioned.Interface) *ResourceManager {
	informerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, DefaultInformerResyncPeriod)
	kosmosInformerFactory := externalversions.NewSharedInformerFactory(kosmosClient, DefaultInformerResyncPeriod)
	endpointSliceInformer := informerFactory.Discovery().V1().EndpointSlices()
	serviceInformer := informerFactory.Core().V1().Services()
	return &ResourceManager{
		InformerFactory:       informerFactory,
		KosmosInformerFactory: kosmosInformerFactory,
		EndpointSliceInformer: endpointSliceInformer.Informer(),
		EndpointSliceLister:   endpointSliceInformer.Lister(),
		ServiceInformer:       serviceInformer.Informer(),
		ServiceLister:         serviceInformer.Lister(),
	}
}
