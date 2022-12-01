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

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

func TestNewBinding(t *testing.T) {
	var (
		brokerName      = "testbroker"
		brokerUID       = types.UID("broker-test-uid")
		rabbitmqcluster = "testrabbitmqcluster"
		namespace       = "foobar"
	)

	for _, tt := range []struct {
		name    string
		args    *rabbit.BindingArgs
		want    *rabbitv1beta1.Binding
		wantErr string
	}{{
		name: "Creates a binding",
		args: &rabbit.BindingArgs{
			Namespace: namespace,
			Name:      "name",
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitmqcluster,
			},
			RabbitMQVhost: "",
			Labels:        map[string]string{"label": "cool"},
			Owner: metav1.OwnerReference{
				Kind:       "Broker",
				APIVersion: "eventing.knative.dev/v1",
				Name:       brokerName,
				UID:        brokerUID,
			},
			Source:      "source",
			Destination: "destination",
		},
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "name",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Broker",
						APIVersion: "eventing.knative.dev/v1",
						Name:       brokerName,
						UID:        brokerUID,
					},
				},
				Labels: map[string]string{"label": "cool"},
			},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "",
				Source:          "source",
				Destination:     "destination",
				DestinationType: "queue",
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: &runtime.RawExtension{Raw: []byte(`{"x-match":"all"}`)},
			},
		},
	}, {
		name: "Creates a binding in RabbitMQ cluster namespace",
		args: &rabbit.BindingArgs{
			Namespace: namespace,
			Name:      "name",
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitmqcluster,
				Namespace: "single-rabbitmq-cluster",
			},
			RabbitMQVhost: "",
			Labels:        map[string]string{"label": "cool"},
			Owner: metav1.OwnerReference{
				Kind:       "Broker",
				APIVersion: "eventing.knative.dev/v1",
				Name:       brokerName,
				UID:        brokerUID,
			},
			Source:      "source",
			Destination: "destination",
		},
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "name",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Broker",
						APIVersion: "eventing.knative.dev/v1",
						Name:       brokerName,
						UID:        brokerUID,
					},
				},
				Labels: map[string]string{"label": "cool"},
			},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "",
				Source:          "source",
				Destination:     "destination",
				DestinationType: "queue",
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name:      rabbitmqcluster,
					Namespace: "single-rabbitmq-cluster",
				},
				Arguments: &runtime.RawExtension{Raw: []byte(`{"x-match":"all"}`)},
			},
		},
	}, {
		name: "appends to filters if given",
		args: &rabbit.BindingArgs{
			RabbitMQVhost: "",
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitmqcluster,
				Namespace: "single-rabbitmq-cluster",
			},
			Filters: map[string]string{"filter": "this"},
		},
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{}}},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "",
				DestinationType: "queue",
				Arguments:       &runtime.RawExtension{Raw: []byte(`{"filter":"this","x-match":"all"}`)},
			},
		},
	}, {
		name: "vhost set",
		args: &rabbit.BindingArgs{
			RabbitMQVhost: "test-vhost",
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitmqcluster,
				Namespace: "single-rabbitmq-cluster",
			},
		},
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{}}},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "test-vhost",
				DestinationType: "queue",
			},
		},
	},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rabbit.NewBinding(tt.args)
			if err != nil && tt.wantErr != "" {
				t.Errorf("Got unexpected error return from NewBinding, wanted %v got %v", tt.wantErr, err)
			} else if err == nil && tt.wantErr != "" {
				t.Errorf("Got unexpected error return from NewBinding, wanted %v got %v", tt.wantErr, err)
			} else if err != nil && (err.Error() != tt.wantErr) {
				t.Errorf("Got unexpected error return from NewBinding, wanted %v got %v", tt.wantErr, err)
			}
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Binding resource: want:\n%+v\ngot:\n%+v\ndiff:\n%+v", tt.want, got, cmp.Diff(tt.want, got))
			}
		})
	}
}
