/*
Copyright 2019 The Knative Authors

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
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqChannel is a resource representing a Rabbitmq Channel.
type RabbitmqChannel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Channel.
	Spec RabbitmqChannelSpec `json:"spec,omitempty"`

	// Status represents the current state of the RabbitmqChannel. This data may be out of
	// date.
	// +optional
	Status RabbitmqChannelStatus `json:"status,omitempty"`
}

// Check that Channel can be validated, can be defaulted, and has immutable fields.
var (
	_ apis.Validatable = (*RabbitmqChannel)(nil)
	_ apis.Defaultable = (*RabbitmqChannel)(nil)
	// Check that InMemoryChannel can return its spec untyped.
	_ apis.HasSpec = (*RabbitmqChannel)(nil)
	// Check that we can create OwnerReferences to an InMemoryChannel.
	_ kmeta.OwnerRefable = (*RabbitmqChannel)(nil)
	_ runtime.Object     = (*RabbitmqChannel)(nil)
	_ duckv1.KRShaped    = (*RabbitmqChannel)(nil)
)

// RabbitmqChannelSpec defines the specification for a RabbitmqChannel.
type RabbitmqChannelSpec struct {
	// Config is a KReference to the configuration that specifies
	// configuration options for this Channel. This is either a pointer
	// to a secret or a reference to RabbitMQCluster from which the connection
	// details are fetched from.
	Config duckv1.KReference `json:"config"`

	// inherits duck/v1 ChannelableSpec, which currently provides:
	// * SubscribableSpec - List of subscribers
	// * DeliverySpec - contains options controlling the event delivery
	eventingduckv1.ChannelableSpec `json:",inline"`
}

// RabbitmqChannelStatus represents the current state of a RabbitmqChannel.
type RabbitmqChannelStatus struct {
	// inherits duck/v1 ChannelableStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	// * AddressStatus is the part where the Channelable fulfills the Addressable contract.
	// * Subscribers is populated with the statuses of each of the Channelable's subscribers.
	// * DeadLetterChannel is a KReference and is set by the channel when it supports native error handling via a channel
	//   Failed messages are delivered here.
	eventingduckv1.ChannelableStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqChannelList is a collection of RabbitmqChannels.
type RabbitmqChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RabbitmqChannel `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for RabbitmqChannels
func (*RabbitmqChannel) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("RabbitmqChannel")
}

// GetStatus retrieves the duck status for this resource. Implements the KRShaped interface.
func (n *RabbitmqChannel) GetStatus() *duckv1.Status {
	return &n.Status.Status
}
