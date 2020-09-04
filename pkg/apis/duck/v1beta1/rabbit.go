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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
)

// +genduck

var _ duck.Implementable = (*Rabbit)(nil)

// At the moment, a Rabbit is the subset of RabbitmqCluster from https://github.com/rabbitmq/cluster-operator/

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Rabbit is a skeleton type wrapping limited fields from RabbitmqCluster.
// This is not a real resource.
type Rabbit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// RabbitSpec
	Spec RabbitSpec `json:"spec"`

	// RabbitmqClusterStatus
	Status RabbitStatus `json:"status"`
}

// RabbiSpec
type RabbitSpec struct{}

// RabbitStatus.
type RabbitStatus struct {
	// RabbitAdmin identifies information on internal resources.
	Admin *RabbitAdmin `json:"admin,omitempty"`
}

type RabbitAdmin struct {
	SecretReference  *RabbitReference `json:"secretReference,omitempty"`
	ServiceReference *RabbitReference `json:"serviceReference,omitempty"`
}

type RabbitReference struct {
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Keys      map[string]string `json:"keys,omitempty"`
}

var (
	// Verify RabbitmqClusterType resources meet duck contracts.
	_ duck.Populatable = (*Rabbit)(nil)
	_ apis.Listable    = (*Rabbit)(nil)
)

// GetFullType implements duck.Implementable
func (s *Rabbit) GetFullType() duck.Populatable {
	return &Rabbit{}
}

// Populate implements duck.Populatable
func (c *Rabbit) Populate() {
	// TODO: fill this out.
}

// GetListType implements apis.Listable
func (c *Rabbit) GetListType() runtime.Object {
	return &RabbitList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitList is a list of Rabbit resources
type RabbitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Rabbit `json:"items"`
}
