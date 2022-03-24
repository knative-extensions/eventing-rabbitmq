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
	v1 "k8s.io/api/core/v1"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

const (
	namespace   = "foobar"
	triggerName = "my-trigger"
	sourceName = "a-source"
	connectionSecretName = "a-random-secret"
)

func TestNewQueue(t *testing.T) {
	owner := metav1.OwnerReference{
		Kind:       "Broker",
		APIVersion: "eventing.knative.dev/v1",
		Name:       brokerName,
		UID:        brokerUID,
	}

	for _, tt := range []struct {
		name    string
		args    *resources.QueueArgs
		want    *rabbitv1beta1.Queue
		wantErr string
	}{
		{
			name: "creates a queue",
			args: &resources.QueueArgs{
				Name:                triggerName,
				Namespace:           namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Owner:               owner,
				Labels:              map[string]string{"cool": "label"},
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
				},
			},
		},
		{
			name: "creates a queue in RabbitMQ cluster namespace",
			args: &resources.QueueArgs{
				Name:                     triggerName,
				Namespace:                namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
					Namespace: "single-rabbitmq-cluster",
				},
				Owner:                    owner,
				Labels:                   map[string]string{"cool": "label"},
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
			name: "creates a queue for source",
			args: &resources.QueueArgs{
				Name:                     sourceName,
				Namespace:                namespace,
				RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
					ConnectionSecret: &v1.LocalObjectReference{
						Name: connectionSecretName,
					},
				},
				Owner:                    owner,
				Labels:                   map[string]string{"a-key": "a-value"},
				Source: &v1alpha1.RabbitmqSource{
					Spec:       v1alpha1.RabbitmqSourceSpec{
						QueueConfig: v1alpha1.RabbitmqSourceQueueConfigSpec{
							Name:       "a-test-queue",
							Durable:    false,
							AutoDelete: false,
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
					Durable:    false,
					AutoDelete: false,
					RabbitmqClusterReference:  rabbitv1beta1.RabbitmqClusterReference{
						ConnectionSecret: &v1.LocalObjectReference{
							Name: connectionSecretName,
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := resources.NewQueue(tt.args)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Queue resource: want:\n%+v\ngot:\n%+v\ndiff:\n%+v", tt.want, got, cmp.Diff(tt.want, got))
			}
		})
	}
}
