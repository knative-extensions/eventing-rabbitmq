package resources_test

import (
	"context"
	"fmt"
	"testing"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/testrabbit"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	brokerName      = "testbroker"
	namespace       = "foobar"
	rabbitmqcluster = "testrabbitmqcluster"
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
		want:      "foobar.testbroker",
	}, {
		name:      brokerName,
		namespace: namespace,
		want:      "foobar.testbroker.dlx",
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

func TestNewExchange(t *testing.T) {
	want := &rabbitv1beta1.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "foobar.testbroker",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "Broker",
					APIVersion: "eventing.knative.dev/v1",
					Name:       brokerName,
				},
			},
			Labels: map[string]string{"eventing.knative.dev/broker": "testbroker"},
		},
		Spec: rabbitv1beta1.ExchangeSpec{
			Name:       "foobar.testbroker",
			Type:       "headers",
			Durable:    true,
			AutoDelete: false,
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitmqcluster,
			},
		},
	}
	args := &resources.ExchangeArgs{
		Broker: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
			},
		},
		RabbitMQCluster: rabbitmqcluster,
	}
	got := resources.NewExchange(context.TODO(), args)
	if !equality.Semantic.DeepDerivative(want, got) {
		t.Errorf("Unespected Exchange resource: want:\n%+v\ngot:\n%+v", want, got)
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
