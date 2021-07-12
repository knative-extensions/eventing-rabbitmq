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
	"testing"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/channel/controller/resources"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	channelName     = "testchannel"
	channelUID      = "channel-test-uid"
	subscriptionUID = "subscription-test-uid"
	namespace       = "foobar"
	rabbitmqcluster = "testrabbitmqcluster"
)

func TestNewExchange(t *testing.T) {
	for _, tt := range []struct {
		name       string
		subscriber *eventingduckv1.SubscriberSpec
		dlx        bool
		want       *rabbitv1beta1.Exchange
	}{{
		name: "channel exchange",
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "c.foobar.testchannel.channel-test-uid",
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
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       "c.foobar.testchannel.channel-test-uid",
				Type:       "fanout",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}, {
		name: "channel DLX exchange",
		dlx:  true,
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "c.foobar.testchannel.dlx.channel-test-uid",
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
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       "c.foobar.testchannel.dlx.channel-test-uid",
				Type:       "fanout",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}, {
		name:       "Subscription DLX exchange",
		dlx:        true,
		subscriber: &eventingduckv1.SubscriberSpec{UID: subscriptionUID},
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "s.foobar.testchannel.dlx.subscription-test-uid",
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
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       "s.foobar.testchannel.dlx.subscription-test-uid",
				Type:       "fanout",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			args := &resources.ExchangeArgs{
				Channel: &messagingv1beta1.RabbitmqChannel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      channelName,
						Namespace: namespace,
						UID:       channelUID,
					},
					Spec: messagingv1beta1.RabbitmqChannelSpec{
						Config: duckv1.KReference{
							Name: rabbitmqcluster,
						},
					},
				},
				RabbitMQCluster: rabbitmqcluster,
				Subscriber:      tt.subscriber,
				DLX:             tt.dlx,
			}

			got := resources.NewExchange(context.TODO(), args)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Exchange resource: want:\n%+v\ngot:\n%+v", tt.want, got)
			}
		})
	}
}
