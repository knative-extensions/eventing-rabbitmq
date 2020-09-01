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
	"knative.dev/eventing-rabbitmq/pkg/reconciler/io"

	"github.com/streadway/amqp"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
)

// ExchangeArgs are the arguments to create a RabbitMQ Exchange.
type ExchangeArgs struct {
	Broker      *eventingv1beta1.Broker
	RabbitmqURL string
}

// DeclareExchange declares the Exchange for a Broker.
func DeclareExchange(args *ExchangeArgs) error {
	conn, err := amqp.Dial(args.RabbitmqURL)
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
	return channel.ExchangeDeclare(
		exchangeName,
		"headers", // kind
		true,      // durable
		false,     // auto-delete
		false,     // internal
		false,     // nowait
		nil,       // args
	)
}

// DeleteExchange deletes the Exchange for a Broker.
func DeleteExchange(args *ExchangeArgs) error {
	conn, err := amqp.Dial(args.RabbitmqURL)
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
