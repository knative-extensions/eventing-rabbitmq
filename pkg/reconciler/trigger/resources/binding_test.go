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
	"fmt"
	"testing"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/testrabbit"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
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
				Name:      "foobar.testbroker.dlq",
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
				Source:          "foobar.testbroker.dlx",
				Destination:     "foobar.testbroker.dlq",
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
				Name:      "foobar.my-trigger",
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
				Source:          "foobar.testbroker",
				Destination:     "foobar.my-trigger",
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
				Name:      "foobar.my-trigger",
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
				Source:          "foobar.testbroker",
				Destination:     "foobar.my-trigger",
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

func TestBindingDeclaration(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := "queue-and-a"
	qualifiedQueueName := namespace + "-" + queueName
	testrabbit.CreateDurableQueue(t, ctx, rabbitContainer, qualifiedQueueName)
	brokerName := "some-broker"
	exchangeName := namespace + "." + brokerName
	testrabbit.CreateExchange(t, ctx, rabbitContainer, exchangeName, "headers")

	err := resources.MakeBinding(nil, &resources.BindingArgs{
		RoutingKey:             "some-key",
		BrokerURL:              testrabbit.BrokerUrl(t, ctx, rabbitContainer).String(),
		RabbitmqManagementPort: testrabbit.ManagementPort(t, ctx, rabbitContainer),
		Trigger: &eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      queueName,
				Namespace: namespace,
			},
			Spec: eventingv1.TriggerSpec{
				Broker: brokerName,
				Filter: &eventingv1.TriggerFilter{
					Attributes: map[string]string{},
				},
			},
		},
	})

	assert.NilError(t, err)
	createdBindings := testrabbit.FindBindings(t, ctx, rabbitContainer)
	assert.Equal(t, len(createdBindings), 2, "Expected 2 bindings: default + requested one")
	defaultBinding := createdBindings[0]
	assert.Equal(t, defaultBinding["source"], "", "Expected binding to default exchange")
	assert.Equal(t, defaultBinding["destination_type"], "queue")
	assert.Equal(t, defaultBinding["destination"], qualifiedQueueName)
	explicitBinding := createdBindings[1]
	assert.Equal(t, explicitBinding["source"], exchangeName)
	assert.Equal(t, explicitBinding["destination_type"], "queue")
	assert.Equal(t, explicitBinding["destination"], qualifiedQueueName)
	assert.Equal(t, asMap(t, explicitBinding["arguments"])[resources.BindingKey], queueName)
}

func TestBindingDLQDeclaration(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := "queue-and-a"
	testrabbit.CreateDurableQueue(t, ctx, rabbitContainer, queueName)
	brokerName := "some-broker"
	exchangeName := namespace + "." + brokerName + ".dlx"
	testrabbit.CreateExchange(t, ctx, rabbitContainer, exchangeName, "headers")

	err := resources.MakeDLQBinding(nil, &resources.BindingArgs{
		RoutingKey:             "some-key",
		BrokerURL:              testrabbit.BrokerUrl(t, ctx, rabbitContainer).String(),
		RabbitmqManagementPort: testrabbit.ManagementPort(t, ctx, rabbitContainer),
		Broker: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
			},
		},
		QueueName: queueName,
	})

	assert.NilError(t, err)
	createdBindings := testrabbit.FindBindings(t, ctx, rabbitContainer)
	assert.Equal(t, len(createdBindings), 2, "Expected 2 bindings: default + requested one")
	defaultBinding := createdBindings[0]
	assert.Equal(t, defaultBinding["source"], "", "Expected binding to default exchange")
	assert.Equal(t, defaultBinding["destination_type"], "queue")
	assert.Equal(t, defaultBinding["destination"], queueName)
	explicitBinding := createdBindings[1]
	assert.Equal(t, explicitBinding["source"], exchangeName)
	assert.Equal(t, explicitBinding["destination_type"], "queue")
	assert.Equal(t, explicitBinding["destination"], queueName)
	assert.Equal(t, asMap(t, explicitBinding["arguments"])[resources.BindingKey], brokerName)
}

func TestMissingExchangeBindingDeclarationFailure(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := "queue-te"
	brokerName := "some-broke-herr"

	brokerURL := testrabbit.BrokerUrl(t, ctx, rabbitContainer).String()

	err := resources.MakeBinding(nil, &resources.BindingArgs{
		RoutingKey:             "some-key",
		BrokerURL:              brokerURL,
		RabbitmqManagementPort: testrabbit.ManagementPort(t, ctx, rabbitContainer),
		Trigger: &eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      queueName,
				Namespace: namespace,
			},
			Spec: eventingv1.TriggerSpec{
				Broker: brokerName,
				Filter: &eventingv1.TriggerFilter{
					Attributes: map[string]string{},
				},
			},
		},
	})

	assert.ErrorContains(t, err, `Failed to declare Binding: Error 404 (not_found): no exchange 'foobar.some-broke-herr' in vhost '/'`)
	assert.ErrorContains(t, err, fmt.Sprintf("no exchange '%s.%s'", namespace, brokerName))
}

func asMap(t *testing.T, value interface{}) map[string]interface{} {
	result, ok := value.(map[string]interface{})
	assert.Equal(t, ok, true)
	return result
}

func getBrokerArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":           "all",
		"x-knative-trigger": brokerName,
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
