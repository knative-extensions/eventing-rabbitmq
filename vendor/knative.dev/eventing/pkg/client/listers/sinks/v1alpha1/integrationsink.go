/*
Copyright 2021 The Knative Authors

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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1alpha1 "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
)

// IntegrationSinkLister helps list IntegrationSinks.
// All objects returned here must be treated as read-only.
type IntegrationSinkLister interface {
	// List lists all IntegrationSinks in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.IntegrationSink, err error)
	// IntegrationSinks returns an object that can list and get IntegrationSinks.
	IntegrationSinks(namespace string) IntegrationSinkNamespaceLister
	IntegrationSinkListerExpansion
}

// integrationSinkLister implements the IntegrationSinkLister interface.
type integrationSinkLister struct {
	indexer cache.Indexer
}

// NewIntegrationSinkLister returns a new IntegrationSinkLister.
func NewIntegrationSinkLister(indexer cache.Indexer) IntegrationSinkLister {
	return &integrationSinkLister{indexer: indexer}
}

// List lists all IntegrationSinks in the indexer.
func (s *integrationSinkLister) List(selector labels.Selector) (ret []*v1alpha1.IntegrationSink, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.IntegrationSink))
	})
	return ret, err
}

// IntegrationSinks returns an object that can list and get IntegrationSinks.
func (s *integrationSinkLister) IntegrationSinks(namespace string) IntegrationSinkNamespaceLister {
	return integrationSinkNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// IntegrationSinkNamespaceLister helps list and get IntegrationSinks.
// All objects returned here must be treated as read-only.
type IntegrationSinkNamespaceLister interface {
	// List lists all IntegrationSinks in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.IntegrationSink, err error)
	// Get retrieves the IntegrationSink from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.IntegrationSink, error)
	IntegrationSinkNamespaceListerExpansion
}

// integrationSinkNamespaceLister implements the IntegrationSinkNamespaceLister
// interface.
type integrationSinkNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all IntegrationSinks in the indexer for a given namespace.
func (s integrationSinkNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.IntegrationSink, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.IntegrationSink))
	})
	return ret, err
}

// Get retrieves the IntegrationSink from the indexer for a given namespace and name.
func (s integrationSinkNamespaceLister) Get(name string) (*v1alpha1.IntegrationSink, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("integrationsink"), name)
	}
	return obj.(*v1alpha1.IntegrationSink), nil
}