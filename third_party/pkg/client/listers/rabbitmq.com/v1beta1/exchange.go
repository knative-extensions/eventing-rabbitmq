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
// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

// ExchangeLister helps list Exchanges.
// All objects returned here must be treated as read-only.
type ExchangeLister interface {
	// List lists all Exchanges in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.Exchange, err error)
	// Exchanges returns an object that can list and get Exchanges.
	Exchanges(namespace string) ExchangeNamespaceLister
	ExchangeListerExpansion
}

// exchangeLister implements the ExchangeLister interface.
type exchangeLister struct {
	indexer cache.Indexer
}

// NewExchangeLister returns a new ExchangeLister.
func NewExchangeLister(indexer cache.Indexer) ExchangeLister {
	return &exchangeLister{indexer: indexer}
}

// List lists all Exchanges in the indexer.
func (s *exchangeLister) List(selector labels.Selector) (ret []*v1beta1.Exchange, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Exchange))
	})
	return ret, err
}

// Exchanges returns an object that can list and get Exchanges.
func (s *exchangeLister) Exchanges(namespace string) ExchangeNamespaceLister {
	return exchangeNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ExchangeNamespaceLister helps list and get Exchanges.
// All objects returned here must be treated as read-only.
type ExchangeNamespaceLister interface {
	// List lists all Exchanges in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.Exchange, err error)
	// Get retrieves the Exchange from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1beta1.Exchange, error)
	ExchangeNamespaceListerExpansion
}

// exchangeNamespaceLister implements the ExchangeNamespaceLister
// interface.
type exchangeNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Exchanges in the indexer for a given namespace.
func (s exchangeNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.Exchange, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Exchange))
	})
	return ret, err
}

// Get retrieves the Exchange from the indexer for a given namespace and name.
func (s exchangeNamespaceLister) Get(name string) (*v1beta1.Exchange, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("exchange"), name)
	}
	return obj.(*v1beta1.Exchange), nil
}
