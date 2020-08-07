/*
Copyright 2020 the original author or authors.

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

package fake

import (
	v1alpha1 "knative.dev/eventing-rabbitmq/broker/pkg/internal/thirdparty/keda/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTriggerAuthentications implements TriggerAuthenticationInterface
type FakeTriggerAuthentications struct {
	Fake *FakeKedaV1alpha1
	ns   string
}

var triggerauthenticationsResource = schema.GroupVersionResource{Group: "keda.k8s.io", Version: "v1alpha1", Resource: "triggerauthentications"}

var triggerauthenticationsKind = schema.GroupVersionKind{Group: "keda.k8s.io", Version: "v1alpha1", Kind: "TriggerAuthentication"}

// Get takes name of the triggerAuthentication, and returns the corresponding triggerAuthentication object, and an error if there is any.
func (c *FakeTriggerAuthentications) Get(name string, options v1.GetOptions) (result *v1alpha1.TriggerAuthentication, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(triggerauthenticationsResource, c.ns, name), &v1alpha1.TriggerAuthentication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TriggerAuthentication), err
}

// List takes label and field selectors, and returns the list of TriggerAuthentications that match those selectors.
func (c *FakeTriggerAuthentications) List(opts v1.ListOptions) (result *v1alpha1.TriggerAuthenticationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(triggerauthenticationsResource, triggerauthenticationsKind, c.ns, opts), &v1alpha1.TriggerAuthenticationList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.TriggerAuthenticationList{ListMeta: obj.(*v1alpha1.TriggerAuthenticationList).ListMeta}
	for _, item := range obj.(*v1alpha1.TriggerAuthenticationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested triggerAuthentications.
func (c *FakeTriggerAuthentications) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(triggerauthenticationsResource, c.ns, opts))

}

// Create takes the representation of a triggerAuthentication and creates it.  Returns the server's representation of the triggerAuthentication, and an error, if there is any.
func (c *FakeTriggerAuthentications) Create(triggerAuthentication *v1alpha1.TriggerAuthentication) (result *v1alpha1.TriggerAuthentication, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(triggerauthenticationsResource, c.ns, triggerAuthentication), &v1alpha1.TriggerAuthentication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TriggerAuthentication), err
}

// Update takes the representation of a triggerAuthentication and updates it. Returns the server's representation of the triggerAuthentication, and an error, if there is any.
func (c *FakeTriggerAuthentications) Update(triggerAuthentication *v1alpha1.TriggerAuthentication) (result *v1alpha1.TriggerAuthentication, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(triggerauthenticationsResource, c.ns, triggerAuthentication), &v1alpha1.TriggerAuthentication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TriggerAuthentication), err
}

// Delete takes name of the triggerAuthentication and deletes it. Returns an error if one occurs.
func (c *FakeTriggerAuthentications) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(triggerauthenticationsResource, c.ns, name), &v1alpha1.TriggerAuthentication{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeTriggerAuthentications) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(triggerauthenticationsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.TriggerAuthenticationList{})
	return err
}

// Patch applies the patch and returns the patched triggerAuthentication.
func (c *FakeTriggerAuthentications) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.TriggerAuthentication, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(triggerauthenticationsResource, c.ns, name, pt, data, subresources...), &v1alpha1.TriggerAuthentication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TriggerAuthentication), err
}
