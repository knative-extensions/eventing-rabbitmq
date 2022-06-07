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

package rabbit_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestNewExchange(t *testing.T) {
	var (
		brokerName       = "testbroker"
		brokerUID        = types.UID("broker-test-uid")
		triggerName      = "testtrigger"
		triggerUID       = types.UID("trigger-test-uid")
		sourceName       = "a-source"
		sourceUID        = types.UID("source-test-uid")
		namespace        = "foobar"
		rabbitmqcluster  = "testrabbitmqcluster"
		connectionSecret = "secret-name"
	)

	for _, tt := range []struct {
		name string
		args *rabbit.ExchangeArgs
		want *rabbitv1beta1.Exchange
	}{{
		name: "broker exchange",
		args: &rabbit.ExchangeArgs{
			Name:      brokerName,
			Namespace: namespace,
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitmqcluster,
			},
			Broker: &eventingv1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      brokerName,
					Namespace: namespace,
					UID:       brokerUID,
				},
				Spec: eventingv1.BrokerSpec{
					Config: &duckv1.KReference{
						Name: rabbitmqcluster,
					},
				},
			},
		},
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Broker",
						APIVersion: "eventing.knative.dev/v1",
						Name:       brokerName,
						UID:        brokerUID,
					},
				},
				Labels: map[string]string{"eventing.knative.dev/broker": "testbroker"},
			},
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       brokerName,
				Type:       "headers",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}, {
		name: "broker exchange in RabbitMQ cluster namespace",
		args: &rabbit.ExchangeArgs{
			Name:      brokerName,
			Namespace: namespace,
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitmqcluster,
				Namespace: "single-rabbitmq-cluster",
			},
			Broker: &eventingv1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      brokerName,
					Namespace: namespace,
					UID:       brokerUID,
				},
				Spec: eventingv1.BrokerSpec{
					Config: &duckv1.KReference{
						Name: rabbitmqcluster,
					},
				},
			},
		},
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Broker",
						APIVersion: "eventing.knative.dev/v1",
						Name:       brokerName,
						UID:        brokerUID,
					},
				},
				Labels: map[string]string{"eventing.knative.dev/broker": "testbroker"},
			},
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       brokerName,
				Type:       "headers",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name:      rabbitmqcluster,
					Namespace: "single-rabbitmq-cluster",
				},
			},
		},
	}, {
		name: "source exchange",
		args: &rabbit.ExchangeArgs{
			Name:      sourceName,
			Namespace: namespace,
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				ConnectionSecret: &corev1.LocalObjectReference{
					Name: connectionSecret,
				},
			},
			Source: &v1alpha1.RabbitmqSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourceName,
					Namespace: namespace,
					UID:       sourceUID,
				},
				Spec: v1alpha1.RabbitmqSourceSpec{
					RabbitmqResourcesConfig: &v1alpha1.RabbitmqResourcesConfigSpec{
						ExchangeName: "some-exchange",
						Vhost:        "test",
					},
				},
			},
		},
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sourceName,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:               "RabbitmqSource",
						APIVersion:         "sources.knative.dev/v1alpha1",
						Name:               sourceName,
						UID:                sourceUID,
						Controller:         pointer.Bool(true),
						BlockOwnerDeletion: pointer.Bool(true),
					},
				},
				Labels: map[string]string{
					"eventing.knative.dev/SourceName": sourceName,
				},
			},
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       "some-exchange",
				Vhost:      "/",
				Type:       "headers",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					ConnectionSecret: &corev1.LocalObjectReference{
						Name: connectionSecret,
					},
				},
			},
		},
	}, {
		name: "trigger exchange",
		args: &rabbit.ExchangeArgs{
			Name:      brokerName,
			Namespace: namespace,
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitmqcluster,
			},
			Broker: &eventingv1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      brokerName,
					Namespace: namespace,
					UID:       brokerUID,
				},
				Spec: eventingv1.BrokerSpec{
					Config: &duckv1.KReference{
						Name: rabbitmqcluster,
					},
				},
			},
			Trigger: &eventingv1.Trigger{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      triggerName,
					UID:       triggerUID,
				},
			},
		},
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Trigger",
						APIVersion: "eventing.knative.dev/v1",
						Name:       triggerName,
						UID:        triggerUID,
					},
				},
				Labels: map[string]string{
					"eventing.knative.dev/broker":  "testbroker",
					"eventing.knative.dev/trigger": "testtrigger",
				},
			},
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       brokerName,
				Type:       "headers",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got := rabbit.NewExchange(tt.args)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Exchange resource: want:\n%+v\ngot:\n%+v\ndiff:\n%+v", tt.want, got, cmp.Diff(tt.want, got))
			}
		})
	}
}
