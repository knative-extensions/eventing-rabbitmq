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
	"fmt"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	eventingv1alpha1 "knative.dev/eventing-rabbitmq/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

func TestNewQueue(t *testing.T) {
	var (
		namespace  = "foobar"
		brokerName = "testbroker"
		sourceName = "a-source"
		owner      = metav1.OwnerReference{
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1",
			Name:       brokerName,
			UID:        brokerUID,
		}
	)

	for _, tt := range []struct {
		name    string
		args    *rabbit.QueueArgs
		want    *rabbitv1beta1.Queue
		wantErr string
	}{
		{
			name: "creates a queue",
			args: &rabbit.QueueArgs{
				Name:      triggerName,
				Namespace: namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Owner:  owner,
				Labels: map[string]string{"cool": "label"},
			},
			want: &rabbitv1beta1.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:            triggerName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{owner},
					Labels:          map[string]string{"cool": "label"},
				},
				Spec: rabbitv1beta1.QueueSpec{
					Name:       triggerName,
					Durable:    true,
					AutoDelete: false,
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						Name: rabbitmqcluster,
					},
					Type: string(eventingv1alpha1.QuorumQueueType),
				},
			},
		},
		{
			name: "creates a queue in RabbitMQ cluster namespace",
			args: &rabbit.QueueArgs{
				Name:      triggerName,
				Namespace: namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name:      rabbitmqcluster,
					Namespace: "single-rabbitmq-cluster",
				},
				Owner:  owner,
				Labels: map[string]string{"cool": "label"},
			},
			want: &rabbitv1beta1.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:            triggerName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{owner},
					Labels:          map[string]string{"cool": "label"},
				},
				Spec: rabbitv1beta1.QueueSpec{
					Name:       triggerName,
					Durable:    true,
					AutoDelete: false,
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						Name:      rabbitmqcluster,
						Namespace: "single-rabbitmq-cluster",
					},
				},
			},
		},
		{
			name: "creates a queue with a name different from crd name",
			args: &rabbit.QueueArgs{
				Name:      triggerName,
				QueueName: "a-random-queue-name",
				Namespace: namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Owner:  owner,
				Labels: map[string]string{"cool": "label"},
			},
			want: &rabbitv1beta1.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:            triggerName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{owner},
					Labels:          map[string]string{"cool": "label"},
				},
				Spec: rabbitv1beta1.QueueSpec{
					Name:       "a-random-queue-name",
					Durable:    true,
					AutoDelete: false,
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						Name: rabbitmqcluster,
					},
				},
			},
		},
		{
			name: "creates a classic queue",
			args: &rabbit.QueueArgs{
				Name:      triggerName,
				Namespace: namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Owner:     owner,
				Labels:    map[string]string{"cool": "label"},
				QueueType: eventingv1alpha1.ClassicQueueType,
			},
			want: &rabbitv1beta1.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:            triggerName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{owner},
					Labels:          map[string]string{"cool": "label"},
				},
				Spec: rabbitv1beta1.QueueSpec{
					Name:       triggerName,
					Durable:    true,
					AutoDelete: false,
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						Name: rabbitmqcluster,
					},
					Type: string(eventingv1alpha1.ClassicQueueType),
				},
			},
		},
		{
			name: "creates a queue for source",
			args: &rabbit.QueueArgs{
				Name:      sourceName,
				Namespace: namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					ConnectionSecret: &v1.LocalObjectReference{
						Name: connectionSecretName,
					},
				},
				Owner:  owner,
				Labels: map[string]string{"a-key": "a-value"},
				Source: &v1alpha1.RabbitmqSource{
					Spec: v1alpha1.RabbitmqSourceSpec{
						RabbitmqResourcesConfig: &v1alpha1.RabbitmqResourcesConfigSpec{
							QueueName: "a-test-queue",
							Vhost:     "test",
						},
					},
				},
			},
			want: &rabbitv1beta1.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:            sourceName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{owner},
					Labels:          map[string]string{"a-key": "a-value"},
				},
				Spec: rabbitv1beta1.QueueSpec{
					Name:       "a-test-queue",
					Vhost:      "test",
					Durable:    true,
					AutoDelete: false,
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						ConnectionSecret: &v1.LocalObjectReference{
							Name: connectionSecretName,
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := rabbit.NewQueue(tt.args)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Queue resource: want:\n%+v\ngot:\n%+v\ndiff:\n%+v", tt.want, got, cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestNewPolicy(t *testing.T) {
	var (
		namespace       = "foobar"
		brokerName      = "testbroker"
		brokerUID       = types.UID("broker-test-uid")
		triggerName     = "testtrigger"
		rabbitmqcluster = "testrabbitmqcluster"
		owner           = metav1.OwnerReference{
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1",
			Name:       brokerName,
			UID:        brokerUID,
		}
	)

	for _, tt := range []struct {
		name    string
		args    *rabbit.QueueArgs
		want    *rabbitv1beta1.Policy
		wantErr string
	}{
		{
			name: "creates a policy",
			args: &rabbit.QueueArgs{
				Name:      triggerName,
				Namespace: namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Owner:     owner,
				Labels:    map[string]string{"cool": "label"},
				DLXName:   pointer.String("an-exchange"),
				QueueName: "a-trigger-queue-name",
			},
			want: &rabbitv1beta1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:            triggerName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{owner},
					Labels:          map[string]string{"cool": "label"},
				},
				Spec: rabbitv1beta1.PolicySpec{
					Name:       triggerName,
					Pattern:    fmt.Sprintf("^%s$", regexp.QuoteMeta("a-trigger-queue-name")),
					ApplyTo:    "queues",
					Priority:   1,
					Definition: &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"dead-letter-exchange": %q}`, "an-exchange"))},
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						Name: rabbitmqcluster,
					},
				},
			},
		},
		{
			name: "creates a queue in a provided namespace",
			args: &rabbit.QueueArgs{
				Name:      triggerName,
				Namespace: namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name:      rabbitmqcluster,
					Namespace: "a-namespace",
				},
				Owner:     owner,
				Labels:    map[string]string{"cool": "label"},
				DLXName:   pointer.String("an-exchange"),
				QueueName: "a-trigger-queue-name",
			},
			want: &rabbitv1beta1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:            triggerName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{owner},
					Labels:          map[string]string{"cool": "label"},
				},
				Spec: rabbitv1beta1.PolicySpec{
					Name:       triggerName,
					Pattern:    fmt.Sprintf("^%s$", regexp.QuoteMeta("a-trigger-queue-name")),
					ApplyTo:    "queues",
					Priority:   1,
					Definition: &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"dead-letter-exchange": %q}`, "an-exchange"))},
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						Name:      rabbitmqcluster,
						Namespace: "a-namespace",
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := rabbit.NewPolicy(tt.args)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Policy resource: want:\n%+v\ngot:\n%+v\ndiff:\n%+v", tt.want, got, cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestNewBrokerDLXPolicy(t *testing.T) {
	var (
		namespace       = "foobar"
		brokerName      = "testbroker"
		brokerUID       = types.UID("broker-test-uid")
		rabbitmqcluster = "testrabbitmqcluster"
		owner           = metav1.OwnerReference{
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1",
			Name:       brokerName,
			UID:        brokerUID,
		}
	)

	for _, tt := range []struct {
		name    string
		args    *rabbit.QueueArgs
		want    *rabbitv1beta1.Policy
		wantErr string
	}{
		{
			name: "creates a policy for queues using broker dead letter queue",
			args: &rabbit.QueueArgs{
				Name:      "a-policy-name",
				Namespace: namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name:      rabbitmqcluster,
					Namespace: "a-namespace",
				},
				Owner:     owner,
				Labels:    map[string]string{"cool": "label"},
				DLXName:   pointer.String("an-exchange"),
				BrokerUID: "a-broker-uuid-owfnwdoij",
			},
			want: &rabbitv1beta1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "a-policy-name",
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{owner},
					Labels:          map[string]string{"cool": "label"},
				},
				Spec: rabbitv1beta1.PolicySpec{
					Name:       "a-policy-name",
					Pattern:    "a-broker-uuid-owfnwdoij$",
					ApplyTo:    "queues",
					Priority:   0,
					Definition: &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"dead-letter-exchange": %q}`, "an-exchange"))},
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						Name:      rabbitmqcluster,
						Namespace: "a-namespace",
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := rabbit.NewBrokerDLXPolicy(tt.args)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Policy resource: want:\n%+v\ngot:\n%+v\ndiff:\n%+v", tt.want, got, cmp.Diff(tt.want, got))
			}
		})
	}
}
