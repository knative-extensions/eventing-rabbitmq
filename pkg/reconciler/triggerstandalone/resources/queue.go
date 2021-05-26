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

	"github.com/streadway/amqp"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/io"
	"knative.dev/eventing/pkg/apis/eventing"

	"github.com/NeowayLabs/wabbit"

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

// QueueLabels generates the labels present on the Queue linking the Broker / Trigger to the
// Queue.
func QueueLabels(b *eventingv1.Broker, t *eventingv1.Trigger) map[string]string {
	if t == nil {
		return map[string]string{
			eventing.BrokerLabelKey: b.Name,
		}
	} else {
		return map[string]string{
			eventing.BrokerLabelKey: b.Name,
			TriggerLabelKey:         t.Name,
		}
	}
}

func CreateBrokerDeadLetterQueueName(b *eventingv1.Broker) string {
	// TODO(vaikas): https://github.com/knative-sandbox/eventing-rabbitmq/issues/61
	// return fmt.Sprintf("%s/%s/DLQ", b.Namespace, b.Name)
	return fmt.Sprintf("%s.%s.dlq", b.Namespace, b.Name)
}

func CreateTriggerQueueName(t *eventingv1.Trigger) string {
	// TODO(vaikas): https://github.com/knative-sandbox/eventing-rabbitmq/issues/61
	// return fmt.Sprintf("%s/%s", t.Namespace, t.Name)
	return fmt.Sprintf("%s.%s", t.Namespace, t.Name)
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
