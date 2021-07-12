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
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/channel/controller/resources"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

func TestNewQueue(t *testing.T) {
	for _, tt := range []struct {
		name       string
		channel    *messagingv1beta1.RabbitmqChannel
		subscriber *eventingduckv1.SubscriberSpec
		want       *rabbitv1beta1.Queue
		wantErr    string
	}{{
		name:    "Channel binding",
		channel: createChannel(),
		want: &rabbitv1beta1.Queue{
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
			Spec: rabbitv1beta1.QueueSpec{
				Name:       "c.foobar.testchannel.dlq.channel-test-uid",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}, {
		name:       "Subscription binding, no delivery",
		channel:    createChannel(),
		subscriber: createSubscriber(),
		want: &rabbitv1beta1.Queue{
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
			Spec: rabbitv1beta1.QueueSpec{
				Name:       "s.foobar.testchannel.subscription-test-uid",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getSubscriberQueueArguments(),
			},
		},
	}, {
		name:       "Subscriber binding, with delivery",
		channel:    createChannel(),
		subscriber: createSubscriberWithDelivery(),
		want: &rabbitv1beta1.Queue{
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
			Spec: rabbitv1beta1.QueueSpec{
				Name:       "s.foobar.testchannel.subscription-test-uid",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getSubscriberQueueArgumentsWithDeadLetterSink(),
			},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got := resources.NewQueue(context.TODO(), tt.channel, tt.subscriber)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Queue resource: want:\n%+v\ngot:\n%+v", tt.want, got)
			}
		})
	}
}

func TestNewSubscriberDLQ(t *testing.T) {
	want := &rabbitv1beta1.Queue{
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
		Spec: rabbitv1beta1.QueueSpec{
			Name:       "s.foobar.testchannel.dlq.subscription-test-uid",
			Durable:    true,
			AutoDelete: false,
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitmqcluster,
			},
		},
	}
	got := resources.NewSubscriptionDLQ(context.TODO(), createChannel(), createSubscriberWithDelivery())
	if !equality.Semantic.DeepDerivative(want, got) {
		t.Errorf("Unexpected Queue resource: want:\n%+v\ngot:\n%+v", want, got)
	}

}

func getSubscriberQueueArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-dead-letter-exchange": naming.ChannelExchangeName(createChannel(), true),
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}

func getSubscriberQueueArgumentsWithDeadLetterSink() *runtime.RawExtension {
	c := &messagingv1beta1.RabbitmqChannel{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: channelName}}

	arguments := map[string]string{
		"x-dead-letter-exchange": naming.SubscriberDLXExchangeName(c, createSubscriberWithDelivery()),
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}
