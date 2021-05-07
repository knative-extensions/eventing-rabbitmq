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

	"gotest.tools/assert"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	brokerresources "knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/testrabbit"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const namespace = "foobar"
const triggerName = "my-trigger"

func TestNewQueue(t *testing.T) {
	for _, tt := range []struct {
		name    string
		broker  *eventingv1.Broker
		trigger *eventingv1.Trigger
		want    *rabbitv1beta1.Queue
		wantErr string
	}{{
		name:   "Broker binding",
		broker: createBroker(),
		want: &rabbitv1beta1.Queue{
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
			Spec: rabbitv1beta1.QueueSpec{
				Name:       "foobar.testbroker.dlq",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}, {
		name:    "Trigger binding, no filter",
		broker:  createBroker(),
		trigger: createTrigger(),
		want: &rabbitv1beta1.Queue{
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
			Spec: rabbitv1beta1.QueueSpec{
				Name:       "foobar.my-trigger",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getTriggerQueueArguments(),
			},
		},
	}, {
		name:    "Trigger binding, filter",
		broker:  createBroker(),
		trigger: createTriggerWithFilter(),
		want: &rabbitv1beta1.Queue{
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
			Spec: rabbitv1beta1.QueueSpec{
				Name:       "foobar.my-trigger",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
				Arguments: getTriggerQueueArguments(),
			},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got := resources.NewQueue(context.TODO(), tt.broker, tt.trigger)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Queue resource: want:\n%+v\ngot:\n%+v", tt.want, got)
			}
		})
	}
}

func TestQueueDeclaration(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)

	queue, err := resources.DeclareQueue(dialer.RealDialer, &resources.QueueArgs{
		QueueName: resources.CreateTriggerQueueName(&eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: namespace,
			},
		}),
		RabbitmqURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer).String(),
	})

	assert.NilError(t, err)
	assert.Equal(t, queue.Name(), fmt.Sprintf("%s.%s", namespace, triggerName))
}

func TestQueueDeclarationWithDLX(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)

	queue, err := resources.DeclareQueue(dialer.RealDialer, &resources.QueueArgs{
		QueueName: resources.CreateTriggerQueueName(&eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: namespace,
			},
		}),
		RabbitmqURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer).String(),
		DLX:         "dlq.example.com",
	})

	assert.NilError(t, err)
	assert.Equal(t, queue.Name(), fmt.Sprintf("%s.%s", namespace, triggerName))
}

func TestIncompatibleQueueDeclarationFailure(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := fmt.Sprintf("%s.%s", namespace, triggerName)
	testrabbit.CreateNonDurableQueue(t, ctx, rabbitContainer, queueName)

	_, err := resources.DeclareQueue(dialer.RealDialer, &resources.QueueArgs{
		QueueName: resources.CreateTriggerQueueName(&eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: namespace,
			},
		}),
		RabbitmqURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer).String(),
	})

	assert.ErrorContains(t, err, fmt.Sprintf("inequivalent arg 'durable' for queue '%s'", queueName))
}

func TestQueueDeletion(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := fmt.Sprintf("%s.%s", namespace, triggerName)
	testrabbit.CreateDurableQueue(t, ctx, rabbitContainer, queueName)

	err := resources.DeleteQueue(dialer.RealDialer, &resources.QueueArgs{
		QueueName: resources.CreateTriggerQueueName(&eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: namespace,
			},
		}),
		RabbitmqURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer).String(),
	})

	assert.NilError(t, err)
	queues := testrabbit.FindQueues(t, ctx, rabbitContainer)
	assert.Equal(t, len(queues), 0)
}

func getTriggerQueueArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-dead-letter-exchange": brokerresources.ExchangeName(createBroker(), true),
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}
