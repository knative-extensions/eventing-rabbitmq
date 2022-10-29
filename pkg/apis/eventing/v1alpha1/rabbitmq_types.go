package v1alpha1

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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName={"rmqbrokerconfig"},categories=all;knative;eventing
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqBrokerConfig is the Schema for the RabbitmqBrokerConfig API.
// +k8s:openapi-gen=true
type RabbitmqBrokerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RabbitmqBrokerConfigSpec `json:"spec,omitempty"`
}

var _ runtime.Object = (*RabbitmqBrokerConfig)(nil)
var _ resourcesemantics.GenericCRD = (*RabbitmqBrokerConfig)(nil)
var _ apis.Defaultable = (*RabbitmqBrokerConfig)(nil)
var _ apis.Validatable = (*RabbitmqBrokerConfig)(nil)

type QueueType string

var (
	QuorumQueueType  = QueueType("quorum")
	ClassicQueueType = QueueType("classic")
)

type RabbitmqBrokerConfigSpec struct {
	// RabbitmqClusterReference stores a reference to RabbitmqCluster. This will get used to create resources on the RabbitMQ Broker.
	RabbitmqClusterReference *v1beta1.RabbitmqClusterReference `json:"rabbitmqClusterReference,omitempty"`

	// +optional
	// +kubebuilder:default:=quorum
	// +kubebuilder:validation:Enum=quorum;classic
	QueueType QueueType `json:"queueType"`

	// VHost is the name of the VHost that will be used to set up our sources
	// +optional
	Vhost string `json:"vhost,omitempty"`
}

func (s *RabbitmqBrokerConfig) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("RabbitmqBrokerConfig")
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqBrokerConfigList contains a list of RabbitmqBrokerConfigs.
type RabbitmqBrokerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RabbitmqBrokerConfig `json:"items"`
}
