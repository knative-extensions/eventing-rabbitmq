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
	"fmt"
	"testing"

	"knative.dev/eventing-rabbitmq/pkg/reconciler/internal/testrabbit"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
)

const namespace = "foobar"
const triggerName = "my-queue"

func TestQueueDeclaration(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)

	queue, err := resources.DeclareQueue(&resources.QueueArgs{
		Trigger: &eventingv1beta1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: namespace,
			},
		},
		RabbitmqURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer),
	})

	assert.NilError(t, err)
	assert.Equal(t, queue.Name, fmt.Sprintf("%s/%s", namespace, triggerName))
}

func TestIncompatibleQueueDeclarationFailure(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := fmt.Sprintf("%s/%s", namespace, triggerName)
	testrabbit.CreateNonDurableQueue(t, ctx, rabbitContainer, queueName)

	_, err := resources.DeclareQueue(&resources.QueueArgs{
		Trigger: &eventingv1beta1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: namespace,
			},
		},
		RabbitmqURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer),
	})

	assert.ErrorContains(t, err, fmt.Sprintf("inequivalent arg 'durable' for queue '%s'", queueName))
}

func TestQueueDeletion(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := fmt.Sprintf("%s/%s", namespace, triggerName)
	testrabbit.CreateDurableQueue(t, ctx, rabbitContainer, queueName)

	err := resources.DeleteQueue(&resources.QueueArgs{
		Trigger: &eventingv1beta1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: namespace,
			},
		},
		RabbitmqURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer),
	})

	assert.NilError(t, err)
	queues := testrabbit.FindQueues(t, ctx, rabbitContainer)
	assert.Equal(t, len(queues), 0)
}
