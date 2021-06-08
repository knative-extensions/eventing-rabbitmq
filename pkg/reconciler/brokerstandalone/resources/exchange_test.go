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
	"knative.dev/eventing-rabbitmq/pkg/reconciler/brokerstandalone/resources"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/testrabbit"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	brokerName  = "testbroker"
	triggerName = "testtrigger"
	namespace   = "foobar"
)

func TestExchangeName(t *testing.T) {
	for _, tt := range []struct {
		name      string
		namespace string
		dlx       bool
		want      string
	}{{
		name:      brokerName,
		namespace: namespace,
		want:      "broker.foobar.testbroker",
	}, {
		name:      brokerName,
		namespace: namespace,
		want:      "broker.foobar.testbroker.dlx",
		dlx:       true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resources.ExchangeName(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Namespace: tt.namespace, Name: tt.name}}, tt.dlx)
			if got != tt.want {
				t.Errorf("Unexpected name for %s/%s DLX: %t: want:\n%+s\ngot:\n%+s", tt.namespace, tt.name, tt.dlx, tt.want, got)
			}
		})
	}
}

func TestTriggerDLXExchangeName(t *testing.T) {
	want := "trigger.foobar.testtrigger.dlx"
	got := resources.TriggerDLXExchangeName(&eventingv1.Trigger{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: triggerName}})
	if want != got {
		t.Errorf("Unexpected name for %s/%s trigger DLX: want:\n%+s\ngot:\n%+s", namespace, triggerName, want, got)
	}
}
func TestExchangeDeclaration(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	brokerName := "x-change"

	_, err := resources.DeclareExchange(dialer.RealDialer, &resources.ExchangeArgs{
		Broker: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
			},
		},
		RabbitMQURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer),
	})

	assert.NilError(t, err)
	exchanges := testrabbit.FindOwnedExchanges(t, ctx, rabbitContainer)
	assert.Equal(t, len(exchanges), 1)
	createdExchange := exchanges[0]
	assert.Equal(t, createdExchange["durable"], true)
	assert.Equal(t, createdExchange["auto_delete"], false)
	assert.Equal(t, createdExchange["internal"], false)
	assert.Equal(t, createdExchange["type"], "headers")
	assert.Equal(t, createdExchange["name"], fmt.Sprintf("broker.%s.%s", namespace, brokerName))
}

func TestIncompatibleExchangeDeclarationFailure(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	brokerName := "x-change"
	exchangeName := fmt.Sprintf("broker.%s.%s", namespace, brokerName)
	testrabbit.CreateExchange(t, ctx, rabbitContainer, exchangeName, "direct")

	_, err := resources.DeclareExchange(dialer.RealDialer, &resources.ExchangeArgs{
		Broker: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
			},
		},
		RabbitMQURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer),
	})

	assert.ErrorContains(t, err, fmt.Sprintf("inequivalent arg 'type' for exchange '%s'", exchangeName))
}

func TestExchangeDeletion(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	brokerName := "x-change"
	exchangeName := fmt.Sprintf("broker.%s.%s", namespace, brokerName)
	testrabbit.CreateExchange(t, ctx, rabbitContainer, exchangeName, "headers")

	err := resources.DeleteExchange(&resources.ExchangeArgs{
		Broker: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
			},
		},
		RabbitMQURL: testrabbit.BrokerUrl(t, ctx, rabbitContainer),
	})

	assert.NilError(t, err)
	exchanges := testrabbit.FindOwnedExchanges(t, ctx, rabbitContainer)
	assert.Equal(t, len(exchanges), 0)
}
