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

package resources_test

import (
	"context"
	"fmt"
	"testing"

	"gotest.tools/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/testrabbit"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/triggerstandalone/resources"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const namespace = "foobar"
const triggerName = "my-trigger"

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
	assert.Equal(t, queue.Name(), fmt.Sprintf("trigger.%s.%s", namespace, triggerName))
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
	assert.Equal(t, queue.Name(), fmt.Sprintf("broker.%s.%s", namespace, triggerName))
}

func TestIncompatibleQueueDeclarationFailure(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := fmt.Sprintf("trigger.%s.%s", namespace, triggerName)
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
	queueName := fmt.Sprintf("trigger.%s.%s", namespace, triggerName)
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
