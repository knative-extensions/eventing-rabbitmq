package resources_test

import (
	"context"
	"fmt"
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/testrabbit"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const namespace = "foobar"

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
	assert.Equal(t, createdExchange["name"], fmt.Sprintf("%s.%s", namespace, brokerName))
}

func TestIncompatibleExchangeDeclarationFailure(t *testing.T) {
	ctx := context.Background()
	rabbitContainer := testrabbit.AutoStartRabbit(t, ctx)
	defer testrabbit.TerminateContainer(t, ctx, rabbitContainer)
	brokerName := "x-change"
	exchangeName := fmt.Sprintf("%s.%s", namespace, brokerName)
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
	exchangeName := fmt.Sprintf("%s.%s", namespace, brokerName)
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
