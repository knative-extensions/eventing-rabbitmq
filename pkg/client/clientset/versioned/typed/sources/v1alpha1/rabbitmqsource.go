/*
Copyright 2022 The Knative Authors

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	scheme "knative.dev/eventing-rabbitmq/pkg/client/clientset/versioned/scheme"
)

// RabbitmqSourcesGetter has a method to return a RabbitmqSourceInterface.
// A group's client should implement this interface.
type RabbitmqSourcesGetter interface {
	RabbitmqSources(namespace string) RabbitmqSourceInterface
}

// RabbitmqSourceInterface has methods to work with RabbitmqSource resources.
type RabbitmqSourceInterface interface {
	Create(ctx context.Context, rabbitmqSource *v1alpha1.RabbitmqSource, opts v1.CreateOptions) (*v1alpha1.RabbitmqSource, error)
	Update(ctx context.Context, rabbitmqSource *v1alpha1.RabbitmqSource, opts v1.UpdateOptions) (*v1alpha1.RabbitmqSource, error)
	UpdateStatus(ctx context.Context, rabbitmqSource *v1alpha1.RabbitmqSource, opts v1.UpdateOptions) (*v1alpha1.RabbitmqSource, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.RabbitmqSource, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.RabbitmqSourceList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.RabbitmqSource, err error)
	RabbitmqSourceExpansion
}

// rabbitmqSources implements RabbitmqSourceInterface
type rabbitmqSources struct {
	client rest.Interface
	ns     string
}

// newRabbitmqSources returns a RabbitmqSources
func newRabbitmqSources(c *SourcesV1alpha1Client, namespace string) *rabbitmqSources {
	return &rabbitmqSources{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the rabbitmqSource, and returns the corresponding rabbitmqSource object, and an error if there is any.
func (c *rabbitmqSources) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.RabbitmqSource, err error) {
	result = &v1alpha1.RabbitmqSource{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("rabbitmqsources").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of RabbitmqSources that match those selectors.
func (c *rabbitmqSources) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.RabbitmqSourceList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.RabbitmqSourceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("rabbitmqsources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested rabbitmqSources.
func (c *rabbitmqSources) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("rabbitmqsources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a rabbitmqSource and creates it.  Returns the server's representation of the rabbitmqSource, and an error, if there is any.
func (c *rabbitmqSources) Create(ctx context.Context, rabbitmqSource *v1alpha1.RabbitmqSource, opts v1.CreateOptions) (result *v1alpha1.RabbitmqSource, err error) {
	result = &v1alpha1.RabbitmqSource{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("rabbitmqsources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(rabbitmqSource).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a rabbitmqSource and updates it. Returns the server's representation of the rabbitmqSource, and an error, if there is any.
func (c *rabbitmqSources) Update(ctx context.Context, rabbitmqSource *v1alpha1.RabbitmqSource, opts v1.UpdateOptions) (result *v1alpha1.RabbitmqSource, err error) {
	result = &v1alpha1.RabbitmqSource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("rabbitmqsources").
		Name(rabbitmqSource.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(rabbitmqSource).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *rabbitmqSources) UpdateStatus(ctx context.Context, rabbitmqSource *v1alpha1.RabbitmqSource, opts v1.UpdateOptions) (result *v1alpha1.RabbitmqSource, err error) {
	result = &v1alpha1.RabbitmqSource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("rabbitmqsources").
		Name(rabbitmqSource.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(rabbitmqSource).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the rabbitmqSource and deletes it. Returns an error if one occurs.
func (c *rabbitmqSources) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("rabbitmqsources").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *rabbitmqSources) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("rabbitmqsources").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched rabbitmqSource.
func (c *rabbitmqSources) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.RabbitmqSource, err error) {
	result = &v1alpha1.RabbitmqSource{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("rabbitmqsources").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
