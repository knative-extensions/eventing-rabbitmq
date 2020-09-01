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

var (
	brokerURL    = os.Getenv("BROKER_URL")
	exchangeName = os.Getenv("EXCHANGE_NAME")
	channel      *amqp.Channel
)

func main() {
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

	c, err := cloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create cloudevents client, %v", err)
	}

	log.Fatal(c.StartReceiver(context.Background(), func(event cloudevents.Event) {
		log.Printf("received: %s", event)
		bytes, err := json.Marshal(event)
		if err != nil {
			log.Fatalf("failed to marshal event, %v", err)
		}
		headers := amqp.Table{
			"type":    event.Type(),
			"source":  event.Source(),
			"subject": event.Subject(),
		}
		for key, val := range event.Extensions() {
			headers[key] = val
		}
		err = channel.Publish(
			exchangeName,
			"",    // routing key
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				Headers:     headers,
				ContentType: "application/json",
				Body:        bytes,
			})
		if err != nil {
			log.Fatal("failed to publish message")
		}
	}))
}
