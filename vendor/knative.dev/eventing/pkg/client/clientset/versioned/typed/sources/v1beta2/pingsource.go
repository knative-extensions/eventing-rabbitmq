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

// Code generated by client-gen. DO NOT EDIT.

package v1beta2

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
	v1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
	scheme "knative.dev/eventing/pkg/client/clientset/versioned/scheme"
)

// PingSourcesGetter has a method to return a PingSourceInterface.
// A group's client should implement this interface.
type PingSourcesGetter interface {
	PingSources(namespace string) PingSourceInterface
}

// PingSourceInterface has methods to work with PingSource resources.
type PingSourceInterface interface {
	Create(ctx context.Context, pingSource *v1beta2.PingSource, opts v1.CreateOptions) (*v1beta2.PingSource, error)
	Update(ctx context.Context, pingSource *v1beta2.PingSource, opts v1.UpdateOptions) (*v1beta2.PingSource, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, pingSource *v1beta2.PingSource, opts v1.UpdateOptions) (*v1beta2.PingSource, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta2.PingSource, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta2.PingSourceList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta2.PingSource, err error)
	PingSourceExpansion
}

// pingSources implements PingSourceInterface
type pingSources struct {
	*gentype.ClientWithList[*v1beta2.PingSource, *v1beta2.PingSourceList]
}

// newPingSources returns a PingSources
func newPingSources(c *SourcesV1beta2Client, namespace string) *pingSources {
	return &pingSources{
		gentype.NewClientWithList[*v1beta2.PingSource, *v1beta2.PingSourceList](
			"pingsources",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *v1beta2.PingSource { return &v1beta2.PingSource{} },
			func() *v1beta2.PingSourceList { return &v1beta2.PingSourceList{} }),
	}
}
