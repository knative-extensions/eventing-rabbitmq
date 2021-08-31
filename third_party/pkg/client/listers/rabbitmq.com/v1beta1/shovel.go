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

// ShovelLister helps list Shovels.
// All objects returned here must be treated as read-only.
type ShovelLister interface {
	// List lists all Shovels in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.Shovel, err error)
	// Shovels returns an object that can list and get Shovels.
	Shovels(namespace string) ShovelNamespaceLister
	ShovelListerExpansion
}

// shovelLister implements the ShovelLister interface.
type shovelLister struct {
	indexer cache.Indexer
}

// NewShovelLister returns a new ShovelLister.
func NewShovelLister(indexer cache.Indexer) ShovelLister {
	return &shovelLister{indexer: indexer}
}

// List lists all Shovels in the indexer.
func (s *shovelLister) List(selector labels.Selector) (ret []*v1beta1.Shovel, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Shovel))
	})
	return ret, err
}

// Shovels returns an object that can list and get Shovels.
func (s *shovelLister) Shovels(namespace string) ShovelNamespaceLister {
	return shovelNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ShovelNamespaceLister helps list and get Shovels.
// All objects returned here must be treated as read-only.
type ShovelNamespaceLister interface {
	// List lists all Shovels in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.Shovel, err error)
	// Get retrieves the Shovel from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1beta1.Shovel, error)
	ShovelNamespaceListerExpansion
}

// shovelNamespaceLister implements the ShovelNamespaceLister
// interface.
type shovelNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Shovels in the indexer for a given namespace.
func (s shovelNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.Shovel, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Shovel))
	})
	return ret, err
}

// Get retrieves the Shovel from the indexer for a given namespace and name.
func (s shovelNamespaceLister) Get(name string) (*v1beta1.Shovel, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("shovel"), name)
	}
	return obj.(*v1beta1.Shovel), nil
}
