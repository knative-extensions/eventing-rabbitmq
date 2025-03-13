/*
Copyright The cert-manager Authors.

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

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeCertificates implements CertificateInterface
type FakeCertificates struct {
	Fake *FakeCertmanagerV1
	ns   string
}

var certificatesResource = v1.SchemeGroupVersion.WithResource("certificates")

var certificatesKind = v1.SchemeGroupVersion.WithKind("Certificate")

// Get takes name of the certificate, and returns the corresponding certificate object, and an error if there is any.
func (c *FakeCertificates) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.Certificate, err error) {
	emptyResult := &v1.Certificate{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(certificatesResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.Certificate), err
}

// List takes label and field selectors, and returns the list of Certificates that match those selectors.
func (c *FakeCertificates) List(ctx context.Context, opts metav1.ListOptions) (result *v1.CertificateList, err error) {
	emptyResult := &v1.CertificateList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(certificatesResource, certificatesKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.CertificateList{ListMeta: obj.(*v1.CertificateList).ListMeta}
	for _, item := range obj.(*v1.CertificateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested certificates.
func (c *FakeCertificates) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(certificatesResource, c.ns, opts))

}

// Create takes the representation of a certificate and creates it.  Returns the server's representation of the certificate, and an error, if there is any.
func (c *FakeCertificates) Create(ctx context.Context, certificate *v1.Certificate, opts metav1.CreateOptions) (result *v1.Certificate, err error) {
	emptyResult := &v1.Certificate{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(certificatesResource, c.ns, certificate, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.Certificate), err
}

// Update takes the representation of a certificate and updates it. Returns the server's representation of the certificate, and an error, if there is any.
func (c *FakeCertificates) Update(ctx context.Context, certificate *v1.Certificate, opts metav1.UpdateOptions) (result *v1.Certificate, err error) {
	emptyResult := &v1.Certificate{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(certificatesResource, c.ns, certificate, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.Certificate), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeCertificates) UpdateStatus(ctx context.Context, certificate *v1.Certificate, opts metav1.UpdateOptions) (result *v1.Certificate, err error) {
	emptyResult := &v1.Certificate{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(certificatesResource, "status", c.ns, certificate, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.Certificate), err
}

// Delete takes name of the certificate and deletes it. Returns an error if one occurs.
func (c *FakeCertificates) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(certificatesResource, c.ns, name, opts), &v1.Certificate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCertificates) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(certificatesResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1.CertificateList{})
	return err
}

// Patch applies the patch and returns the patched certificate.
func (c *FakeCertificates) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Certificate, err error) {
	emptyResult := &v1.Certificate{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(certificatesResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.Certificate), err
}
