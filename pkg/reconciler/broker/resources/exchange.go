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

package resources

import (
	"context"
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"github.com/NeowayLabs/wabbit"
	rabbitv1alpha2 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	"github.com/streadway/amqp"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/io"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

// ExchangeArgs are the arguments to create a RabbitMQ Exchange.
type ExchangeArgs struct {
	Broker          *eventingv1.Broker
	RabbitMQURL     *url.URL
	RabbitMQCluster string
	// Set to true to create a DLX, which basically just means we're going
	// to create it with a /DLX as the prepended name.
	DLX bool
}

func NewExchange(ctx context.Context, args *ExchangeArgs) *rabbitv1alpha2.Exchange {
	exchangeName := ExchangeName(args.Broker, args.DLX)
	return &rabbitv1alpha2.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      exchangeName,
			//				fmt.Sprintf("%s-", args.Broker.Name), string(args.Broker.GetUID())),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Broker),
			},
			Labels: ExchangeLabels(args.Broker),
		},
		Spec: rabbitv1alpha2.ExchangeSpec{
			// Why is the name in the Spec again? Is this different from the ObjectMeta.Name? If not,
			// maybe it should be removed?
			Name:       exchangeName,
			Type:       "headers",
			Durable:    true,
			AutoDelete: false,
			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			// TODO: This one has to exist in the same namespace as this exchange.
			RabbitmqClusterReference: rabbitv1alpha2.RabbitmqClusterReference{
				Name: args.RabbitMQCluster,
			},
		},
	}
}

// ExchangeLabels generates the labels present on the Exchange linking the Broker to the
// Exchange.
func ExchangeLabels(b *eventingv1.Broker) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker": b.Name,
	}
}

// DeclareExchange declares the Exchange for a Broker.
func DeclareExchange(dialerFunc dialer.DialerFunc, args *ExchangeArgs) (*corev1.Secret, error) {
	conn, err := dialerFunc(args.RabbitMQURL.String())
	if err != nil {
		return nil, err
	}
	defer io.CloseAmqpResourceAndExitOnError(conn)

	channel, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	defer io.CloseAmqpResourceAndExitOnError(channel)

	return MakeSecret(args), channel.ExchangeDeclare(
		ExchangeName(args.Broker, args.DLX),
		"headers", // kind
		wabbit.Option{
			"durable":    true,
			"autoDelete": false,
			"internal":   false,
			"noWait":     false,
		},
	)
}

// DeleteExchange deletes the Exchange for a Broker.
func DeleteExchange(args *ExchangeArgs) error {
	conn, err := amqp.Dial(args.RabbitMQURL.String())
	if err != nil {
		return err
	}
	defer io.CloseAmqpResourceAndExitOnError(conn)

	channel, err := conn.Channel()
	if err != nil {
		return err
	}
	defer io.CloseAmqpResourceAndExitOnError(channel)

	return channel.ExchangeDelete(
		ExchangeName(args.Broker, args.DLX),
		false, // if-unused
		false, // nowait
	)
}

// ExchangeName constructs a name given a Broker.
// Format is broker.Namespace.broker.Name for normal exchanges and
// broker.Namespace.broker.Name.DLX for DLX exchanges.
func ExchangeName(b *eventingv1.Broker, DLX bool) string {
	var exchangeBase string
	if DLX {
		exchangeBase = fmt.Sprintf("%s.%s.dlx", b.Namespace, b.Name)
	} else {
		exchangeBase = fmt.Sprintf("%s.%s", b.Namespace, b.Name)

	}
	foo := kmeta.ChildName(exchangeBase, string(b.GetUID()))
	fmt.Printf("TODO: Fix this and use consistently to avoid collisions, worth doing? %s\n", foo)
	return exchangeBase
}
