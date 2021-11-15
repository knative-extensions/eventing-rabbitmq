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
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	brokerName      = "testbroker"
	brokerUID       = "broker-test-uid"
	rabbitmqcluster = "testrabbitmqcluster"
)

func TestNewBinding(t *testing.T) {
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerName,
			Namespace: namespace,
			UID:       brokerUID,
		},
		Spec: eventingv1.BrokerSpec{
			Config: &duckv1.KReference{
				Name:      rabbitmqcluster,
				Namespace: namespace,
			},
		},
	}
	for _, tt := range []struct {
		name    string
		args    *resources.BindingArgs
		want    *rabbitv1beta1.Binding
		wantErr string
	}{
		{
			name: "Creates a binding",
			args: &resources.BindingArgs{
				Namespace: namespace,
				Name:      "name",
				Broker:    broker,
				Labels:    map[string]string{"label": "cool"},
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
					Vhost:           "/",
					Source:          "source",
					Destination:     "destination",
					DestinationType: "queue",
					RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
						Name: rabbitmqcluster,
					},
					Arguments: &runtime.RawExtension{Raw: []byte(`{"x-match":"all"}`)},
				},
			},
		},
		{
			name: "appends to filters if given",
			args: &resources.BindingArgs{
				Broker:  broker,
				Filters: map[string]string{"filter": "this"},
			},
			want: &rabbitv1beta1.Binding{
				ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{}}},
				Spec: rabbitv1beta1.BindingSpec{
					Vhost:           "/",
					DestinationType: "queue",
					Arguments:       &runtime.RawExtension{Raw: []byte(`{"filter":"this","x-match":"all"}`)},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resources.NewBinding(context.TODO(), tt.args)
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
