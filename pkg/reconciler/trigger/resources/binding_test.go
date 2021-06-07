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
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	v1 "knative.dev/pkg/apis/duck/v1"
)

const (
	brokerName      = "testbroker"
	rabbitmqcluster = "testrabbitmqcluster"
)

func TestNewBinding(t *testing.T) {
	for _, tt := range []struct {
		name    string
		broker  *eventingv1.Broker
		trigger *eventingv1.Trigger
		want    *rabbitv1beta1.Binding
		wantErr string
	}{{
		name:   "Broker binding",
		broker: createBroker(),
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "broker.foobar.testbroker.dlq",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Broker",
						APIVersion: "eventing.knative.dev/v1",
						Name:       brokerName,
					},
				},
				Labels: map[string]string{"eventing.knative.dev/broker": "testbroker"},
			},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "/",
				Source:          "broker.foobar.testbroker.dlx",
				Destination:     "broker.foobar.testbroker.dlq",
				DestinationType: "queue",
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getBrokerArguments(),
			},
		},
	}, {
		name:    "Trigger binding, no filter",
		broker:  createBroker(),
		trigger: createTrigger(),
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "trigger.foobar.my-trigger",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Trigger",
						APIVersion: "eventing.knative.dev/v1",
						Name:       triggerName,
					},
				},
				Labels: map[string]string{
					"eventing.knative.dev/broker":  "testbroker",
					"eventing.knative.dev/trigger": "my-trigger",
				},
			},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "/",
				Source:          "broker.foobar.testbroker",
				Destination:     "trigger.foobar.my-trigger",
				DestinationType: "queue",
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getTriggerArguments(),
			},
		},
	}, {
		name:    "Trigger binding, filter",
		broker:  createBroker(),
		trigger: createTriggerWithFilter(),
		want: &rabbitv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "trigger.foobar.my-trigger",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Trigger",
						APIVersion: "eventing.knative.dev/v1",
						Name:       triggerName,
					},
				},
				Labels: map[string]string{
					"eventing.knative.dev/broker":  "testbroker",
					"eventing.knative.dev/trigger": "my-trigger",
				},
			},
			Spec: rabbitv1beta1.BindingSpec{
				Vhost:           "/",
				Source:          "broker.foobar.testbroker",
				Destination:     "trigger.foobar.my-trigger",
				DestinationType: "queue",
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getTriggerWithFilterArguments(),
			},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resources.NewBinding(context.TODO(), tt.broker, tt.trigger)
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
			Name:      "trigger.foobar.my-trigger.dlq",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "Trigger",
					APIVersion: "eventing.knative.dev/v1",
					Name:       triggerName,
				},
			},
			Labels: map[string]string{
				"eventing.knative.dev/broker":  "testbroker",
				"eventing.knative.dev/trigger": "my-trigger",
			},
		},
		Spec: rabbitv1beta1.BindingSpec{
			Vhost:           "/",
			Source:          "trigger.foobar.my-trigger.dlx",
			Destination:     "trigger.foobar.my-trigger.dlq",
			DestinationType: "queue",
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitmqcluster,
			},
			Arguments: getTriggerWithFilterArgumentsDLQ(),
		},
	}
	got, err := resources.NewTriggerDLQBinding(context.TODO(), createBroker(), createTriggerWithFilterAndDelivery())
	if err != nil {
		t.Error("NewTriggerDLQBinding failed: ", err)
	}
	if !equality.Semantic.DeepDerivative(want, got) {
		t.Errorf("Unexpected Trigger DLQ Binding resource: want:\n%+v\ngot:\n%+v", want, got)
	}

}

func createBroker() *eventingv1.Broker {
	return &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      brokerName,
		},
		Spec: eventingv1.BrokerSpec{
			Config: &v1.KReference{
				Name: rabbitmqcluster,
			},
		},
	}
}

func createTrigger() *eventingv1.Trigger {
	return &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
		},
		Spec: eventingv1.TriggerSpec{},
	}
}

func createTriggerWithFilter() *eventingv1.Trigger {
	return &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
		},
		Spec: eventingv1.TriggerSpec{
			Filter: &eventingv1.TriggerFilter{
				Attributes: map[string]string{
					"source": "mysourcefilter",
				},
			},
		},
	}
}

func createTriggerWithFilterAndDelivery() *eventingv1.Trigger {
	return &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
		},
		Spec: eventingv1.TriggerSpec{
			Filter: &eventingv1.TriggerFilter{
				Attributes: map[string]string{
					"source": "mysourcefilter",
				},
			},
			Delivery: &eventingduckv1.DeliverySpec{
				DeadLetterSink: &v1.Destination{
					URI: apis.HTTP("example.com"),
				},
			},
		},
	}
}

func getBrokerArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":       "all",
		"x-knative-dlq": brokerName,
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}

func getTriggerArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":           "all",
		"x-knative-trigger": triggerName,
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}

func getTriggerWithFilterArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":           "all",
		"x-knative-trigger": triggerName,
		"source":            "mysourcefilter",
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}

func getTriggerWithFilterArgumentsDLQ() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":               "all",
		"x-knative-trigger-dlq": triggerName,
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}
