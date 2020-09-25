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

	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/io"

	"github.com/NeowayLabs/wabbit"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

// QueueArgs are the arguments to create a Trigger's RabbitMQ Queue.
type QueueArgs struct {
	Trigger     *eventingv1.Trigger
	RabbitmqURL string
	// If non-empty, wire the queue into this DLX.
	DLX string
}

func createQueueName(t *eventingv1.Trigger) string {
	// TODO(vaikas): https://github.com/knative-sandbox/eventing-rabbitmq/issues/61
	// return fmt.Sprintf("%s/%s", t.Namespace, t.Name)
	return fmt.Sprintf("%s-%s", t.Namespace, t.Name)
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
		options["args"] = map[string]interface{}{"x-dead-letter-exchange": args.DLX}
	}

	queue, err := channel.QueueDeclare(
		createQueueName(args.Trigger),
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
		createQueueName(args.Trigger),
		wabbit.Option{
			"ifUnused": false,
			"ifEmpty":  false,
			"noWait":   false,
		},
	)
	return err
}
