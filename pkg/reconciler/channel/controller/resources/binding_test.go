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

package resources_test

import (
	"context"
	"encoding/json"
	"testing"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/channel/controller/resources"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
	v1 "knative.dev/pkg/apis/duck/v1"
)

func TestNewBinding(t *testing.T) {
	for _, tt := range []struct {
		name       string
		channel    *messagingv1beta1.RabbitmqChannel
		subscriber *eventingduckv1.SubscriberSpec
		want       *rabbitv1beta1.Binding
		wantErr    string
	}{{
		name:    "Channel binding",
		channel: createChannel(),
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "c.foobar.testchannel.dlq.channel-test-uid",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "RabbitmqChannel",
						APIVersion: "messaging.knative.dev/v1beta1",
						Name:       channelName,
						UID:        channelUID,
					},
				},
				Labels: map[string]string{"messaging.knative.dev/channel": "testchannel"},
			},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "/",
				Source:          "c.foobar.testchannel.dlx.channel-test-uid",
				Destination:     "c.foobar.testchannel.dlq.channel-test-uid",
				DestinationType: "queue",
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getChannelArguments(),
			},
		},
	}, {
		name:       "Subscriber binding, no delivery",
		channel:    createChannel(),
		subscriber: createSubscriber(),
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "s.foobar.testchannel.subscription-test-uid",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "RabbitmqChannel",
						APIVersion: "messaging.knative.dev/v1beta1",
						Name:       channelName,
						UID:        channelUID,
					},
				},
				Labels: map[string]string{
					"messaging.knative.dev/channel":      "testchannel",
					"messaging.knative.dev/subscription": "subscription-test-uid",
				},
			},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "/",
				Source:          "c.foobar.testchannel.channel-test-uid",
				Destination:     "s.foobar.testchannel.subscription-test-uid",
				DestinationType: "queue",
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getSubscriberArguments(),
			},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resources.NewBinding(context.TODO(), tt.channel, tt.subscriber)
			if err != nil && tt.wantErr != "" {
				t.Errorf("Got unexpected error return from NewBinding, wanted %v got %v", tt.wantErr, err)
			} else if err == nil && tt.wantErr != "" {
				t.Errorf("Got unexpected error return from NewBinding, wanted %v got %v", tt.wantErr, err)
			} else if err != nil && (err.Error() != tt.wantErr) {
				t.Errorf("Got unexpected error return from NewBinding, wanted %v got %v", tt.wantErr, err)
			}
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Binding resource: want:\n%+v\ngot:\n%+v", tt.want, got)
			}
		})
	}
}

func TestNewTriggerDLQBinding(t *testing.T) {
	want := &rabbitv1beta1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "s.foobar.testchannel.dlq.subscription-test-uid",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "RabbitmqChannel",
					APIVersion: "messaging.knative.dev/v1beta1",
					Name:       channelName,
					UID:        channelUID,
				},
			},
			Labels: map[string]string{
				"messaging.knative.dev/channel":      "testchannel",
				"messaging.knative.dev/subscription": "subscription-test-uid",
			},
		},
		Spec: rabbitv1beta1.BindingSpec{
			Vhost:           "/",
			Source:          "s.foobar.testchannel.dlx.subscription-test-uid",
			Destination:     "s.foobar.testchannel.dlq.subscription-test-uid",
			DestinationType: "queue",
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitmqcluster,
			},
			Arguments: getSubscriberArgumentsDLQ(),
		},
	}
	got, err := resources.NewSubscriberDLQBinding(context.TODO(), createChannel(), createSubscriberWithDelivery())
	if err != nil {
		t.Error("NewTriggerDLQBinding failed: ", err)
	}
	if !equality.Semantic.DeepDerivative(want, got) {
		t.Errorf("Unexpected Trigger DLQ Binding resource: want:\n%+v\ngot:\n%+v", want, got)
	}

}

func createChannel() *messagingv1beta1.RabbitmqChannel {
	return &messagingv1beta1.RabbitmqChannel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      channelName,
			UID:       channelUID,
		},
		Spec: messagingv1beta1.RabbitmqChannelSpec{
			Config: v1.KReference{
				Name: rabbitmqcluster,
			},
		},
	}
}

func createSubscriber() *eventingduckv1.SubscriberSpec {
	return &eventingduckv1.SubscriberSpec{UID: subscriptionUID}
}

func createSubscriberWithDelivery() *eventingduckv1.SubscriberSpec {
	return &eventingduckv1.SubscriberSpec{
		UID: subscriptionUID,
		Delivery: &eventingduckv1.DeliverySpec{
			DeadLetterSink: &v1.Destination{
				URI: apis.HTTP("example.com"),
			},
		},
	}
}

func getChannelArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":               "all",
		"x-knative-channel-dlq": channelName,
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}

func getSubscriberArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":                "all",
		"x-knative-subscription": "subscription-test-uid",
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}

func getSubscriberArgumentsDLQ() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":                    "all",
		"x-knative-subscription-dlq": subscriptionUID,
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}
