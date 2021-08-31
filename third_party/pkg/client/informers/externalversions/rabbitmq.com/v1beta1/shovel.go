/*
Copyright 2020 The Knative Authors

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

package v1beta1

import (
	"context"
	time "time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	rabbitmqcomv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	versioned "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned"
	internalinterfaces "knative.dev/eventing-rabbitmq/third_party/pkg/client/informers/externalversions/internalinterfaces"
	v1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/client/listers/rabbitmq.com/v1beta1"
)

// ShovelInformer provides access to a shared informer and lister for
// Shovels.
type ShovelInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.ShovelLister
}

type shovelInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewShovelInformer constructs a new informer for Shovel type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewShovelInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredShovelInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredShovelInformer constructs a new informer for Shovel type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredShovelInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RabbitmqV1beta1().Shovels(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RabbitmqV1beta1().Shovels(namespace).Watch(context.TODO(), options)
			},
		},
		&rabbitmqcomv1beta1.Shovel{},
		resyncPeriod,
		indexers,
	)
}

func (f *shovelInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredShovelInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *shovelInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&rabbitmqcomv1beta1.Shovel{}, f.defaultInformer)
}

func (f *shovelInformer) Lister() v1beta1.ShovelLister {
	return v1beta1.NewShovelLister(f.Informer().GetIndexer())
}
