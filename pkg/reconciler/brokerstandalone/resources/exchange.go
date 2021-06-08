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

package resources

import (
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/kmeta"

	"github.com/NeowayLabs/wabbit"
	"github.com/streadway/amqp"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/io"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

// ExchangeArgs are the arguments to create a RabbitMQ Exchange.
type ExchangeArgs struct {
	Broker          *eventingv1.Broker
	Trigger         *eventingv1.Trigger
	RabbitMQURL     *url.URL
	RabbitMQCluster string
	// Set to true to create a DLX, which basically just means we're going
	// to create it with a /DLX as the prepended name.
	DLX bool
}

// ExchangeLabels generates the labels present on the Exchange linking the Broker to the
// Exchange.
func ExchangeLabels(b *eventingv1.Broker, t *eventingv1.Trigger) map[string]string {
	if t != nil {
		return map[string]string{
			"eventing.knative.dev/broker":  b.Name,
			"eventing.knative.dev/trigger": t.Name,
		}
	} else {
		return map[string]string{
			"eventing.knative.dev/broker": b.Name,
		}
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

	var exchangeName string
	if args.Trigger != nil {
		exchangeName = TriggerDLXExchangeName(args.Trigger)
	} else {
		exchangeName = ExchangeName(args.Broker, args.DLX)
	}
	fmt.Printf("DECLARING EXCHANGE WITH NAME: %s", exchangeName)
	return MakeSecret(args), channel.ExchangeDeclare(
		exchangeName,
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

	var exchangeName string
	if args.Trigger != nil {
		exchangeName = TriggerDLXExchangeName(args.Trigger)
	} else {
		exchangeName = ExchangeName(args.Broker, args.DLX)
	}

	return channel.ExchangeDelete(
		exchangeName,
		false, // if-unused
		false, // nowait
	)
}

// ExchangeName constructs a name given a Broker.
// Format is broker.Namespace.broker.Name for normal exchanges and
// broker.Namespace.broker.Name.dlx for DLX exchanges.
func ExchangeName(b *eventingv1.Broker, DLX bool) string {
	var exchangeBase string
	if DLX {
		exchangeBase = fmt.Sprintf("broker.%s.%s.dlx", b.Namespace, b.Name)
	} else {
		exchangeBase = fmt.Sprintf("broker.%s.%s", b.Namespace, b.Name)

	}
	foo := kmeta.ChildName(exchangeBase, string(b.GetUID()))
	fmt.Printf("TODO: Fix this and use consistently to avoid collisions, worth doing? %s\n", foo)
	return exchangeBase
}

// TriggerDLXExchangeName constructs a name given a Broker.
// Format is trigger.Namespace.Name.dlx
func TriggerDLXExchangeName(t *eventingv1.Trigger) string {
	exchangeBase := fmt.Sprintf("trigger.%s.%s.dlx", t.Namespace, t.Name)
	foo := kmeta.ChildName(exchangeBase, string(t.GetUID()))
	fmt.Printf("TODO: Fix this and use consistently to avoid collisions, worth doing? %s\n", foo)
	return exchangeBase
}
