/*
Copyright The Kosmos Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// VirtualClusterPluginInformer provides access to a shared informer and lister for
// VirtualClusterPlugins.
type VirtualClusterPluginInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.VirtualClusterPluginLister
}

type virtualClusterPluginInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewVirtualClusterPluginInformer constructs a new informer for VirtualClusterPlugin type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewVirtualClusterPluginInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredVirtualClusterPluginInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredVirtualClusterPluginInformer constructs a new informer for VirtualClusterPlugin type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredVirtualClusterPluginInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KosmosV1alpha1().VirtualClusterPlugins(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KosmosV1alpha1().VirtualClusterPlugins(namespace).Watch(context.TODO(), options)
			},
		},
		&kosmosv1alpha1.VirtualClusterPlugin{},
		resyncPeriod,
		indexers,
	)
}

func (f *virtualClusterPluginInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredVirtualClusterPluginInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *virtualClusterPluginInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kosmosv1alpha1.VirtualClusterPlugin{}, f.defaultInformer)
}

func (f *virtualClusterPluginInformer) Lister() v1alpha1.VirtualClusterPluginLister {
	return v1alpha1.NewVirtualClusterPluginLister(f.Informer().GetIndexer())
}
