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

// QueueArgs are the arguments to create a Trigger's RabbitMQ Queue.
type QueueArgs struct {
	Trigger     *eventingv1beta1.Trigger
	RabbitmqURL string
}

// DeclareQueue declares the Trigger's Queue.
func DeclareQueue(args *QueueArgs) (*amqp.Queue, error) {
	conn, err := amqp.Dial(args.RabbitmqURL)
	if err != nil {
		return nil, err
	}
	defer io.CloseAmqpResourceAndExitOnError(conn)

	channel, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	defer io.CloseAmqpResourceAndExitOnError(channel)

	queueName := fmt.Sprintf("%s/%s", args.Trigger.Namespace, args.Trigger.Name)
	queue, err := channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // nowait
		nil,   // args
	)
	if err != nil {
		return nil, err
	}
	return &queue, nil
}

// DeleteQueue deletes the Trigger's Queue.
func DeleteQueue(args *QueueArgs) error {
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

	queueName := fmt.Sprintf("%s/%s", args.Trigger.Namespace, args.Trigger.Name)
	_, err = channel.QueueDelete(
		queueName,
		false, // if-unused
		false, // if-empty
		false, // nowait
	)
	return err
}
