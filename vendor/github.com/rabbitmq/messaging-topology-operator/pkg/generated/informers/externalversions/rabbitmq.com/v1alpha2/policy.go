/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.
This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.
This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha2

import (
	"context"
	time "time"

	rabbitmqcomv1alpha2 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	versioned "github.com/rabbitmq/messaging-topology-operator/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/rabbitmq/messaging-topology-operator/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha2 "github.com/rabbitmq/messaging-topology-operator/pkg/generated/listers/rabbitmq.com/v1alpha2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// PolicyInformer provides access to a shared informer and lister for
// Policies.
type PolicyInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha2.PolicyLister
}

type policyInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewPolicyInformer constructs a new informer for Policy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewPolicyInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredPolicyInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredPolicyInformer constructs a new informer for Policy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredPolicyInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RabbitmqV1alpha2().Policies(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RabbitmqV1alpha2().Policies(namespace).Watch(context.TODO(), options)
			},
		},
		&rabbitmqcomv1alpha2.Policy{},
		resyncPeriod,
		indexers,
	)
}

func (f *policyInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredPolicyInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *policyInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&rabbitmqcomv1alpha2.Policy{}, f.defaultInformer)
}

func (f *policyInformer) Lister() v1alpha2.PolicyLister {
	return v1alpha2.NewPolicyLister(f.Informer().GetIndexer())
}
