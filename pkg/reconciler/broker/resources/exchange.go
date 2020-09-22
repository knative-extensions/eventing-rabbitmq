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
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"

	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/io"

	"github.com/NeowayLabs/wabbit"
	"github.com/streadway/amqp"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

// ExchangeArgs are the arguments to create a RabbitMQ Exchange.
type ExchangeArgs struct {
	Broker      *eventingv1.Broker
	RabbitMQURL *url.URL
}

// DeclareExchange declares the Exchange for a Broker.
func DeclareExchange(dialerFunc dialer.DialerFunc, args *ExchangeArgs) (*corev1.Secret, error) {
	//	conn, err := amqp.Dial(args.RabbitMQURL.String())
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

	exchangeName := fmt.Sprintf("%s/%s", args.Broker.Namespace, ExchangeName(args.Broker.Name))
	return MakeSecret(args), channel.ExchangeDeclare(
		exchangeName,
		"headers", // kind
		wabbit.Option{
			"durable":  true,
			"delete":   false,
			"internal": false,
			"noWait":   false,
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

	exchangeName := fmt.Sprintf("%s/%s", args.Broker.Namespace, ExchangeName(args.Broker.Name))
	return channel.ExchangeDelete(
		exchangeName,
		false, // if-unused
		false, // nowait
	)
}

// ExchangeName derives the Exchange name from the Broker name
func ExchangeName(brokerName string) string {
	return fmt.Sprintf("knative-%s", brokerName)
}
