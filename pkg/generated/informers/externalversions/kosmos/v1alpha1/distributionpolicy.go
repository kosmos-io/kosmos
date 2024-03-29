// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	versioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// DistributionPolicyInformer provides access to a shared informer and lister for
// DistributionPolicies.
type DistributionPolicyInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.DistributionPolicyLister
}

type distributionPolicyInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewDistributionPolicyInformer constructs a new informer for DistributionPolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewDistributionPolicyInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredDistributionPolicyInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredDistributionPolicyInformer constructs a new informer for DistributionPolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredDistributionPolicyInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KosmosV1alpha1().DistributionPolicies(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KosmosV1alpha1().DistributionPolicies(namespace).Watch(context.TODO(), options)
			},
		},
		&kosmosv1alpha1.DistributionPolicy{},
		resyncPeriod,
		indexers,
	)
}

func (f *distributionPolicyInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredDistributionPolicyInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *distributionPolicyInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kosmosv1alpha1.DistributionPolicy{}, f.defaultInformer)
}

func (f *distributionPolicyInformer) Lister() v1alpha1.DistributionPolicyLister {
	return v1alpha1.NewDistributionPolicyLister(f.Informer().GetIndexer())
}
