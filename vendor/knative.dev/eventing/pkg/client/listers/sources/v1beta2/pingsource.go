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

package v1beta2

import (
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
	sourcesv1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
)

// PingSourceLister helps list PingSources.
// All objects returned here must be treated as read-only.
type PingSourceLister interface {
	// List lists all PingSources in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*sourcesv1beta2.PingSource, err error)
	// PingSources returns an object that can list and get PingSources.
	PingSources(namespace string) PingSourceNamespaceLister
	PingSourceListerExpansion
}

// pingSourceLister implements the PingSourceLister interface.
type pingSourceLister struct {
	listers.ResourceIndexer[*sourcesv1beta2.PingSource]
}

// NewPingSourceLister returns a new PingSourceLister.
func NewPingSourceLister(indexer cache.Indexer) PingSourceLister {
	return &pingSourceLister{listers.New[*sourcesv1beta2.PingSource](indexer, sourcesv1beta2.Resource("pingsource"))}
}

// PingSources returns an object that can list and get PingSources.
func (s *pingSourceLister) PingSources(namespace string) PingSourceNamespaceLister {
	return pingSourceNamespaceLister{listers.NewNamespaced[*sourcesv1beta2.PingSource](s.ResourceIndexer, namespace)}
}

// PingSourceNamespaceLister helps list and get PingSources.
// All objects returned here must be treated as read-only.
type PingSourceNamespaceLister interface {
	// List lists all PingSources in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*sourcesv1beta2.PingSource, err error)
	// Get retrieves the PingSource from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*sourcesv1beta2.PingSource, error)
	PingSourceNamespaceListerExpansion
}

// pingSourceNamespaceLister implements the PingSourceNamespaceLister
// interface.
type pingSourceNamespaceLister struct {
	listers.ResourceIndexer[*sourcesv1beta2.PingSource]
}
