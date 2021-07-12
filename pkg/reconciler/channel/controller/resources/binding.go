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

package resources

import (
	"context"
	"encoding/json"
	"fmt"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/pkg/kmeta"

	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

const (
	BindingKey              = "x-knative-subscription"
	DLQBindingKey           = "x-knative-channel-dlq"
	SubscriberDLQBindingKey = "x-knative-subscription-dlq"
)

func NewBinding(ctx context.Context, c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) (*rabbitv1beta1.Binding, error) {
	var bindingName string
	var sourceName string

	arguments := map[string]string{
		"x-match": "all",
	}
	// Depending on if we have a Channel & Subscriber we need to rejigger some of the names and
	// configurations little differently, so do that up front before actually creating the resource
	// that we're returning.
	if s != nil {
		bindingName = naming.CreateSubscriberQueueName(c, s)
		arguments[BindingKey] = string(s.UID)
		sourceName = naming.ChannelExchangeName(c, false)
	} else {
		bindingName = naming.CreateChannelDeadLetterQueueName(c)
		arguments[DLQBindingKey] = c.Name
		sourceName = naming.ChannelExchangeName(c, true)
	}

	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to encode binding arguments %+v : %s", argumentsJson, err)
	}

	binding := &rabbitv1beta1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       c.Namespace,
			Name:            bindingName,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(c)},
			Labels:          BindingLabels(c, s),
		},
		Spec: rabbitv1beta1.BindingSpec{
			Vhost:           "/",
			Source:          sourceName,
			Destination:     bindingName,
			DestinationType: "queue",
			RoutingKey:      "",
			Arguments: &runtime.RawExtension{
				Raw: argumentsJson,
			},

			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			// TODO: This one has to exist in the same namespace as this exchange.
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: c.Spec.Config.Name,
			},
		},
	}
	return binding, nil
}

// NewTriggerDLQBinding creates a binding for a Trigger DLX.
func NewSubscriberDLQBinding(ctx context.Context, c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) (*rabbitv1beta1.Binding, error) {

	arguments := map[string]string{
		"x-match":               "all",
		SubscriberDLQBindingKey: string(s.UID),
	}

	bindingName := naming.CreateSubscriberDeadLetterQueueName(c, s)
	sourceName := naming.SubscriberDLXExchangeName(c, s)
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to encode DLQ binding arguments %+v : %s", argumentsJson, err)
	}

	binding := &rabbitv1beta1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       c.Namespace,
			Name:            bindingName,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(c)},
			Labels:          BindingLabels(c, s),
		},
		Spec: rabbitv1beta1.BindingSpec{
			Vhost:           "/",
			Source:          sourceName,
			Destination:     bindingName,
			DestinationType: "queue",
			RoutingKey:      "",
			Arguments: &runtime.RawExtension{
				Raw: argumentsJson,
			},

			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			// TODO: This one has to exist in the same namespace as this exchange.
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: c.Spec.Config.Name,
			},
		},
	}
	return binding, nil
}

// BindingLabels generates the labels present on the Queue linking the Broker / Trigger to the
// Binding.
func BindingLabels(c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) map[string]string {
	if s == nil {
		return map[string]string{
			"messaging.knative.dev/channel": c.Name,
		}
	} else {
		return map[string]string{
			"messaging.knative.dev/channel":      c.Name,
			"messaging.knative.dev/subscription": string(s.UID),
		}
	}
}
