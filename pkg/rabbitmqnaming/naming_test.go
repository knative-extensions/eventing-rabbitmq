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

	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	brokerName  = "testbroker"
	brokerUID   = "brokeruid"
	triggerName = "testtrigger"
	triggerUID  = "triggeruid"
	namespace   = "foobar"
	sourceName  = "a-source"
	sourceUID   = "asourceUID"
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
	want := "t.q.foobar.testtrigger.triggeruid"
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

func TestCreateTriggerQueueRabbitName(t *testing.T) {
	want := "foobar.testtrigger.brokeruid"
	got := CreateTriggerQueueRabbitName(&eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
			UID:       triggerUID,
		},
	}, brokerUID)
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

func TestCreateSourceRabbitName(t *testing.T) {
	want := "s.foobar.a-source.asourceUID"
	got := CreateSourceRabbitName(&v1alpha1.RabbitmqSource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      sourceName,
			UID:       sourceUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for source: want:\n%q\ngot:\n%q", want, got)
	}
}
