/*
Copyright 2020 The KEDA Authors

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

	kedav1alpha1 "github.com/kedacore/keda/api/v1alpha1"
	versioned "github.com/kedacore/keda/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/kedacore/keda/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/kedacore/keda/pkg/generated/listers/keda/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ScaledObjectInformer provides access to a shared informer and lister for
// ScaledObjects.
type ScaledObjectInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.ScaledObjectLister
}

type scaledObjectInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewScaledObjectInformer constructs a new informer for ScaledObject type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewScaledObjectInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredScaledObjectInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredScaledObjectInformer constructs a new informer for ScaledObject type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredScaledObjectInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KedaV1alpha1().ScaledObjects(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KedaV1alpha1().ScaledObjects(namespace).Watch(context.TODO(), options)
			},
		},
		&kedav1alpha1.ScaledObject{},
		resyncPeriod,
		indexers,
	)
}

func (f *scaledObjectInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredScaledObjectInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *scaledObjectInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kedav1alpha1.ScaledObject{}, f.defaultInformer)
}

func (f *scaledObjectInformer) Lister() v1alpha1.ScaledObjectLister {
	return v1alpha1.NewScaledObjectLister(f.Informer().GetIndexer())
}
