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

package fake

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

// FakeSchemaReplications implements SchemaReplicationInterface
type FakeSchemaReplications struct {
	Fake *FakeRabbitmqV1beta1
	ns   string
}

var schemareplicationsResource = schema.GroupVersionResource{Group: "rabbitmq.com", Version: "v1beta1", Resource: "schemareplications"}

var schemareplicationsKind = schema.GroupVersionKind{Group: "rabbitmq.com", Version: "v1beta1", Kind: "SchemaReplication"}

// Get takes name of the schemaReplication, and returns the corresponding schemaReplication object, and an error if there is any.
func (c *FakeSchemaReplications) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.SchemaReplication, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(schemareplicationsResource, c.ns, name), &v1beta1.SchemaReplication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.SchemaReplication), err
}

// List takes label and field selectors, and returns the list of SchemaReplications that match those selectors.
func (c *FakeSchemaReplications) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.SchemaReplicationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(schemareplicationsResource, schemareplicationsKind, c.ns, opts), &v1beta1.SchemaReplicationList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.SchemaReplicationList{ListMeta: obj.(*v1beta1.SchemaReplicationList).ListMeta}
	for _, item := range obj.(*v1beta1.SchemaReplicationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested schemaReplications.
func (c *FakeSchemaReplications) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(schemareplicationsResource, c.ns, opts))

}

// Create takes the representation of a schemaReplication and creates it.  Returns the server's representation of the schemaReplication, and an error, if there is any.
func (c *FakeSchemaReplications) Create(ctx context.Context, schemaReplication *v1beta1.SchemaReplication, opts v1.CreateOptions) (result *v1beta1.SchemaReplication, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(schemareplicationsResource, c.ns, schemaReplication), &v1beta1.SchemaReplication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.SchemaReplication), err
}

// Update takes the representation of a schemaReplication and updates it. Returns the server's representation of the schemaReplication, and an error, if there is any.
func (c *FakeSchemaReplications) Update(ctx context.Context, schemaReplication *v1beta1.SchemaReplication, opts v1.UpdateOptions) (result *v1beta1.SchemaReplication, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(schemareplicationsResource, c.ns, schemaReplication), &v1beta1.SchemaReplication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.SchemaReplication), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeSchemaReplications) UpdateStatus(ctx context.Context, schemaReplication *v1beta1.SchemaReplication, opts v1.UpdateOptions) (*v1beta1.SchemaReplication, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(schemareplicationsResource, "status", c.ns, schemaReplication), &v1beta1.SchemaReplication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.SchemaReplication), err
}

// Delete takes name of the schemaReplication and deletes it. Returns an error if one occurs.
func (c *FakeSchemaReplications) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(schemareplicationsResource, c.ns, name, opts), &v1beta1.SchemaReplication{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeSchemaReplications) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(schemareplicationsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.SchemaReplicationList{})
	return err
}

// Patch applies the patch and returns the patched schemaReplication.
func (c *FakeSchemaReplications) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.SchemaReplication, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(schemareplicationsResource, c.ns, name, pt, data, subresources...), &v1beta1.SchemaReplication{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.SchemaReplication), err
}
