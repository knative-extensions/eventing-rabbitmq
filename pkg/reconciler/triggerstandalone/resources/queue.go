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
	amqp "github.com/rabbitmq/amqp091-go"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/io"

	"wabbit"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const TriggerLabelKey = "eventing.knative.dev/trigger"

// QueueArgs are the arguments to create a Trigger's RabbitMQ Queue.
type QueueArgs struct {
	QueueName       string
	RabbitmqURL     string
	RabbitmqCluster string
	// If the queue is for Trigger, this holds the trigger so we can create a proper Owner Ref
	Trigger *eventingv1.Trigger
	// If non-empty, wire the queue into this DLX.
	DLX string
}

// DeclareQueue declares the Trigger's Queue.
func DeclareQueue(dialerFunc dialer.DialerFunc, args *QueueArgs) (wabbit.Queue, error) {
	conn, err := dialerFunc(args.RabbitmqURL)
	if err != nil {
		return nil, err
	}
	defer io.CloseAmqpResourceAndExitOnError(conn)

	channel, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	defer io.CloseAmqpResourceAndExitOnError(channel)

	options := wabbit.Option{
		"durable":    true,
		"autoDelete": false,
		"exclusive":  false,
		"noWait":     false,
	}
	if args.DLX != "" {
		rabbitArgs := make(map[string]interface{}, 1)
		rabbitArgs["x-dead-letter-exchange"] = interface{}(args.DLX)
		options["args"] = amqp.Table(rabbitArgs)
	}
	queue, err := channel.QueueDeclare(
		args.QueueName,
		options,
	)
	if err != nil {
		return nil, err
	}
	return queue, nil
}

// DeleteQueue deletes the Trigger's Queue.
func DeleteQueue(dialerFunc dialer.DialerFunc, args *QueueArgs) error {
	conn, err := dialerFunc(args.RabbitmqURL)
	if err != nil {
		return err
	}
	defer io.CloseAmqpResourceAndExitOnError(conn)

	channel, err := conn.Channel()
	if err != nil {
		return err
	}
	defer io.CloseAmqpResourceAndExitOnError(channel)

	_, err = channel.QueueDelete(
		args.QueueName,
		wabbit.Option{
			"ifUnused": false,
			"ifEmpty":  false,
			"noWait":   false,
		},
	)
	return err
}
