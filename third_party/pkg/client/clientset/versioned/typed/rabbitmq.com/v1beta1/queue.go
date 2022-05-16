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

package v1beta1

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	scheme "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned/scheme"
)

// QueuesGetter has a method to return a QueueInterface.
// A group's client should implement this interface.
type QueuesGetter interface {
	Queues(namespace string) QueueInterface
}

// QueueInterface has methods to work with Queue resources.
type QueueInterface interface {
	Create(ctx context.Context, queue *v1beta1.Queue, opts v1.CreateOptions) (*v1beta1.Queue, error)
	Update(ctx context.Context, queue *v1beta1.Queue, opts v1.UpdateOptions) (*v1beta1.Queue, error)
	UpdateStatus(ctx context.Context, queue *v1beta1.Queue, opts v1.UpdateOptions) (*v1beta1.Queue, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.Queue, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.QueueList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Queue, err error)
	QueueExpansion
}

// queues implements QueueInterface
type queues struct {
	client rest.Interface
	ns     string
}

// newQueues returns a Queues
func newQueues(c *RabbitmqV1beta1Client, namespace string) *queues {
	return &queues{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the queue, and returns the corresponding queue object, and an error if there is any.
func (c *queues) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.Queue, err error) {
	result = &v1beta1.Queue{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("queues").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Queues that match those selectors.
func (c *queues) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.QueueList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.QueueList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("queues").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested queues.
func (c *queues) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("queues").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a queue and creates it.  Returns the server's representation of the queue, and an error, if there is any.
func (c *queues) Create(ctx context.Context, queue *v1beta1.Queue, opts v1.CreateOptions) (result *v1beta1.Queue, err error) {
	result = &v1beta1.Queue{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("queues").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(queue).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a queue and updates it. Returns the server's representation of the queue, and an error, if there is any.
func (c *queues) Update(ctx context.Context, queue *v1beta1.Queue, opts v1.UpdateOptions) (result *v1beta1.Queue, err error) {
	result = &v1beta1.Queue{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("queues").
		Name(queue.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(queue).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *queues) UpdateStatus(ctx context.Context, queue *v1beta1.Queue, opts v1.UpdateOptions) (result *v1beta1.Queue, err error) {
	result = &v1beta1.Queue{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("queues").
		Name(queue.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(queue).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the queue and deletes it. Returns an error if one occurs.
func (c *queues) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("queues").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *queues) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("queues").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched queue.
func (c *queues) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Queue, err error) {
	result = &v1beta1.Queue{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("queues").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
