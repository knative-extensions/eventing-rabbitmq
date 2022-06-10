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

package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName={"rmqsource"},categories=all;knative;eventing;sources;importers
// +kubebuilder:subresource:status
// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqSource is the Schema for the rabbitmqsources API.
// +k8s:openapi-gen=true
type RabbitmqSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RabbitmqSourceSpec   `json:"spec,omitempty"`
	Status RabbitmqSourceStatus `json:"status,omitempty"`
}

var _ runtime.Object = (*RabbitmqSource)(nil)
var _ resourcesemantics.GenericCRD = (*RabbitmqSource)(nil)
var _ kmeta.OwnerRefable = (*RabbitmqSource)(nil)
var _ apis.Defaultable = (*RabbitmqSource)(nil)
var _ apis.Validatable = (*RabbitmqSource)(nil)
var _ duckv1.KRShaped = (*RabbitmqSource)(nil)

type RabbitmqResourcesConfigSpec struct {
	// VHost is the name of the VHost that will be used to set up our sources
	// +optional
	Vhost string `json:"vhost,omitempty"`
	// Name of the queue to bind to; required value.
	// +required
	QueueName string `json:"queueName"`
	// Name of the exchange; Required when predeclared is false.
	// +optional
	ExchangeName string `json:"exchangeName,omitempty"`
	// Sets the Channel's Prefetch count and number of Workers to consume simultaneously from it
	// +optional
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=1000
	// +kubebuilder:default:=1
	Parallelism *int `json:"parallelism,omitempty"`
	// Predeclared defines if channels and queues are already predeclared and shouldn't be recreated.
	// This should be used in case the user does not have permission to declare new queues and channels in
	// RabbitMQ cluster
	// +optional
	Predeclared bool `json:"predeclared,omitempty"`
}

type DeliverySpec struct {
	// Retry is the minimum number of retries the sender should attempt when
	// sending an event before moving it to the dead letter sink.
	// +optional
	Retry *int32 `json:"retry,omitempty"`
	// BackoffPolicy is the retry backoff policy (linear, exponential).
	// +optional
	BackoffPolicy *eventingduckv1.BackoffPolicyType `json:"backoffPolicy,omitempty"`
	// BackoffDelay is the delay before retrying.
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	//
	// For linear policy, backoff delay is backoffDelay*<numberOfRetries>.
	// For exponential policy, backoff delay is backoffDelay*2^<numberOfRetries>.
	// +optional
	BackoffDelay *string `json:"backoffDelay,omitempty"`
}

type RabbitmqSourceSpec struct {
	// RabbitmqClusterReference stores a reference to RabbitmqCluster. This will get used to create resources on the RabbitMQ Broker.
	// +required
	RabbitmqClusterReference *v1beta1.RabbitmqClusterReference `json:"rabbitmqClusterReference,omitempty"`
	// Delivery contains the delivery spec for each trigger
	// to this Broker. Each trigger delivery spec, if any, overrides this
	// global delivery spec.
	// +optional
	Delivery *DeliverySpec `json:"delivery,omitempty"`
	// RabbitmqResourcesConfig config for Rabbitmq resources: Exchange, Channel and Queue
	// +optional
	RabbitmqResourcesConfig *RabbitmqResourcesConfigSpec `json:"rabbitmqResourcesConfig"`
	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// Required property.
	// +required
	Sink *duckv1.Destination `json:"sink"`
	// ServiceAccountName is the name of the ServiceAccount that will be used to run the Receive
	// Adapter Deployment.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	RabbitmqEventType = "dev.knative.rabbitmq.event"
)

// RabbitmqEventSource returns cloud event attribute 'source' for messages published by a rabbitmqsource
// format is '/apis/v1/namespace/NAMESPACE/rabbitmqsources/NAME#QUEUE_NAME'
func RabbitmqEventSource(namespace, rabbitmqSourceName, qName string) string {
	return fmt.Sprintf("/apis/v1/namespaces/%s/rabbitmqsources/%s#%s", namespace, rabbitmqSourceName, qName)
}

type RabbitmqSourceStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.SourceStatus `json:",inline"`
}

func (s *RabbitmqSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("RabbitmqSource")
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqSourceList contains a list of RabbitmqSources.
type RabbitmqSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RabbitmqSource `json:"items"`
}
