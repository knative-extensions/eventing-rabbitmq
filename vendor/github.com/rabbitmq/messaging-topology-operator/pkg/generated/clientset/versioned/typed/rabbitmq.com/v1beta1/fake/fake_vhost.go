/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeVhosts implements VhostInterface
type FakeVhosts struct {
	Fake *FakeRabbitmqV1beta1
	ns   string
}

var vhostsResource = schema.GroupVersionResource{Group: "rabbitmq.com", Version: "v1beta1", Resource: "vhosts"}

var vhostsKind = schema.GroupVersionKind{Group: "rabbitmq.com", Version: "v1beta1", Kind: "Vhost"}

// Get takes name of the vhost, and returns the corresponding vhost object, and an error if there is any.
func (c *FakeVhosts) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.Vhost, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(vhostsResource, c.ns, name), &v1beta1.Vhost{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Vhost), err
}

// List takes label and field selectors, and returns the list of Vhosts that match those selectors.
func (c *FakeVhosts) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.VhostList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(vhostsResource, vhostsKind, c.ns, opts), &v1beta1.VhostList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.VhostList{ListMeta: obj.(*v1beta1.VhostList).ListMeta}
	for _, item := range obj.(*v1beta1.VhostList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested vhosts.
func (c *FakeVhosts) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(vhostsResource, c.ns, opts))

}

// Create takes the representation of a vhost and creates it.  Returns the server's representation of the vhost, and an error, if there is any.
func (c *FakeVhosts) Create(ctx context.Context, vhost *v1beta1.Vhost, opts v1.CreateOptions) (result *v1beta1.Vhost, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(vhostsResource, c.ns, vhost), &v1beta1.Vhost{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Vhost), err
}

// Update takes the representation of a vhost and updates it. Returns the server's representation of the vhost, and an error, if there is any.
func (c *FakeVhosts) Update(ctx context.Context, vhost *v1beta1.Vhost, opts v1.UpdateOptions) (result *v1beta1.Vhost, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(vhostsResource, c.ns, vhost), &v1beta1.Vhost{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Vhost), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeVhosts) UpdateStatus(ctx context.Context, vhost *v1beta1.Vhost, opts v1.UpdateOptions) (*v1beta1.Vhost, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(vhostsResource, "status", c.ns, vhost), &v1beta1.Vhost{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Vhost), err
}

// Delete takes name of the vhost and deletes it. Returns an error if one occurs.
func (c *FakeVhosts) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(vhostsResource, c.ns, name, opts), &v1beta1.Vhost{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVhosts) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(vhostsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.VhostList{})
	return err
}

// Patch applies the patch and returns the patched vhost.
func (c *FakeVhosts) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Vhost, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(vhostsResource, c.ns, name, pt, data, subresources...), &v1beta1.Vhost{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Vhost), err
}
