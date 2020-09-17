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

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/internal/testrabbit"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

func TestBindingDeclaration(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := "queue-and-a"
	qualifiedQueueName := namespace + "-" + queueName
	testrabbit.CreateDurableQueue(t, ctx, rabbitContainer, qualifiedQueueName)
	brokerName := "some-broker"
	exchangeName := namespace + "/" + "knative-" + brokerName
	testrabbit.CreateExchange(t, ctx, rabbitContainer, exchangeName, "headers")

	err := resources.MakeBinding(&resources.BindingArgs{
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

func TestMissingExchangeBindingDeclarationFailure(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	queueName := "queue-te"
	brokerName := "some-broke-herr"

	brokerURL := testrabbit.BrokerUrl(t, ctx, rabbitContainer).String()

	err := resources.MakeBinding(&resources.BindingArgs{
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

	assert.ErrorContains(t, err, "Failed to declare Binding. Expected 201 response, but got: 404.")
	assert.ErrorContains(t, err, fmt.Sprintf("no exchange '%s/knative-%s'", namespace, brokerName))
}

func asMap(t *testing.T, value interface{}) map[string]interface{} {
	result, ok := value.(map[string]interface{})
	assert.Equal(t, ok, true)
	return result
}
