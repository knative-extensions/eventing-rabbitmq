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

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/streadway/amqp"
)

func main() {
	queueName := os.Getenv("QUEUE_NAME")
	brokerURL := os.Getenv("BROKER_URL")
	brokerIngressURL := os.Getenv("BROKER_INGRESS_URL")
	subscriberURL := os.Getenv("SUBSCRIBER")

	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %s", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open a channel: %s", err)
	}
	defer channel.Close()

	err = channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		log.Fatalf("failed to create QoS: %s", err)
	}

	msgs, err := channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		log.Fatalf("failed to create consumer: %s", err)
	}

	forever := make(chan bool)

	ceClient, err := cloudevents.NewDefaultClient()

	if err != nil {
		log.Fatal("failed to create http client")
	}

	go func() {
		for d := range msgs {
			event := cloudevents.NewEvent()
			err := json.Unmarshal(d.Body, &event)
			if err != nil {
				log.Printf("failed to unmarshal event (nacking and not requeueing): %s", err)
				d.Nack(false, false) // not multiple, do not requeue
				continue
			}

			ctx := cloudevents.ContextWithTarget(context.Background(), subscriberURL)
			response, result := ceClient.Request(ctx, event)
			if !cloudevents.IsACK(result) {
				log.Printf("failed downstream (nacking and requeueing): %s", result.Error())
				d.Nack(false, true) // not multiple, do requeue
				continue
			}
			if response != nil {
				ctx = cloudevents.ContextWithTarget(context.Background(), brokerIngressURL)
				if result := ceClient.Send(ctx, *response); !cloudevents.IsACK(result) {
					d.Nack(false, true) // not multiple, do requeue
				}
			}
			d.Ack(false) // not multiple
		}
	}()

	log.Printf("rabbitmq receiver started, exit with CTRL+C")
	<-forever
}
