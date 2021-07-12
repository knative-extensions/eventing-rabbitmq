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

package naming

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	brokerName      = "testbroker"
	brokerUID       = "brokeruid"
	triggerName     = "testtrigger"
	triggerUID      = "triggeruid"
	namespace       = "foobar"
	channelName     = "testchannel"
	channelUID      = "channeluid"
	subscriptionUID = "subscriptionuid"
)

func TestExchangeName(t *testing.T) {
	for _, tt := range []struct {
		name      string
		namespace string
		dlx       bool
		want      string
	}{{
		name:      brokerName,
		namespace: namespace,
		want:      "b.foobar.testbroker.brokeruid",
	}, {
		name:      brokerName,
		namespace: namespace,
		want:      "b.foobar.testbroker.dlx.brokeruid",
		dlx:       true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got := BrokerExchangeName(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{UID: brokerUID, Namespace: tt.namespace, Name: tt.name}}, tt.dlx)
			if got != tt.want {
				t.Errorf("Unexpected name for Broker Exchange %s/%s DLX: %t: want:\n%+s\ngot:\n%+s", tt.namespace, tt.name, tt.dlx, tt.want, got)
			}
		})
	}
}

func TestTriggerDLXExchangeName(t *testing.T) {
	want := "t.foobar.testtrigger.dlx.triggeruid"
	got := TriggerDLXExchangeName(&eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
			UID:       triggerUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/testtrigger Trigger DLX: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateBrokerDeadLetterQueueName(t *testing.T) {
	want := "b.foobar.testbroker.dlq.brokeruid"
	got := CreateBrokerDeadLetterQueueName(&eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      brokerName,
			UID:       brokerUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/testbroker Broker DLQ: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateTriggerQueueName(t *testing.T) {
	want := "t.foobar.testtrigger.triggeruid"
	got := CreateTriggerQueueName(&eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
			UID:       triggerUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/testtrigger Queue: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateTriggerDeadLetterQueueName(t *testing.T) {
	want := "t.foobar.testtrigger.dlq.triggeruid"
	got := CreateTriggerDeadLetterQueueName(&eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
			UID:       triggerUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/trigger Trigger DLQ: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestChannelExchangeName(t *testing.T) {
	for _, tt := range []struct {
		name      string
		namespace string
		dlx       bool
		want      string
	}{{
		name:      channelName,
		namespace: namespace,
		want:      "c.foobar.testchannel.channeluid",
	}, {
		name:      channelName,
		namespace: namespace,
		want:      "c.foobar.testchannel.dlx.channeluid",
		dlx:       true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got := ChannelExchangeName(&messagingv1beta1.RabbitmqChannel{ObjectMeta: metav1.ObjectMeta{UID: channelUID, Namespace: tt.namespace, Name: tt.name}}, tt.dlx)
			if got != tt.want {
				t.Errorf("Unexpected name for Channel Exchange %s/%s DLX: %t: want:\n%+s\ngot:\n%+s", tt.namespace, tt.name, tt.dlx, tt.want, got)
			}
		})
	}
}

func TestSubscriptionDLXExchangeName(t *testing.T) {
	c := &messagingv1beta1.RabbitmqChannel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      channelName,
			UID:       channelUID,
		},
	}

	want := "s.foobar.testchannel.dlx.subscriptionuid"
	got := SubscriberDLXExchangeName(c, &eventingduckv1.SubscriberSpec{UID: subscriptionUID})
	if want != got {
		t.Errorf("Unexpected name for foobar/testsubscription Subscription DLX: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateChannelDeadLetterQueueName(t *testing.T) {
	want := "c.foobar.testchannel.dlq.channeluid"
	got := CreateChannelDeadLetterQueueName(&messagingv1beta1.RabbitmqChannel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      channelName,
			UID:       channelUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/testchannel Channel DLQ: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateSubscriptionQueueName(t *testing.T) {
	c := &messagingv1beta1.RabbitmqChannel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      channelName,
			UID:       channelUID,
		},
	}

	want := "s.foobar.testchannel.subscriptionuid"
	got := CreateSubscriberQueueName(c, &eventingduckv1.SubscriberSpec{UID: subscriptionUID})
	if want != got {
		t.Errorf("Unexpected name for foobar/testbroker Subscription queue: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateSubscriptionDeadLetterQueueName(t *testing.T) {
	c := &messagingv1beta1.RabbitmqChannel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      channelName,
			UID:       channelUID,
		},
	}

	want := "s.foobar.testchannel.dlq.subscriptionuid"
	got := CreateSubscriberDeadLetterQueueName(c, &eventingduckv1.SubscriberSpec{UID: subscriptionUID})
	if want != got {
		t.Errorf("Unexpected name for foobar/testbroker Subscription DLQ: want:\n%q\ngot:\n%q", want, got)
	}
}
