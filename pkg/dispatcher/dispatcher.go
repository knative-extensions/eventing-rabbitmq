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

package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/NeowayLabs/wabbit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
)

type Dispatcher struct {
	queueName        string
	brokerURL        string
	brokerIngressURL string
	subscriberURL    string
	delivery         *eventingduckv1.DeliverySpec
	dialerFunc       dialer.DialerFunc
}

func NewDispatcher(queueName, brokerURL, brokerIngressURL, subscriberURL string, delivery *eventingduckv1.DeliverySpec) *Dispatcher {
	return &Dispatcher{
		queueName:        queueName,
		brokerURL:        brokerURL,
		brokerIngressURL: brokerIngressURL,
		subscriberURL:    subscriberURL,
		delivery:         delivery,
		dialerFunc:       dialer.RealDialer,
	}

}

func (d *Dispatcher) SetDialerFunc(dialerFunc dialer.DialerFunc) {
	d.dialerFunc = dialerFunc
}

func (d *Dispatcher) Start(ctx context.Context) {
	logging.FromContext(ctx).Infow("Starting dispatcher")
	conn, err := d.dialerFunc(d.brokerURL)
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
		d.queueName, // queue
		"",          // consumer
		wabbit.Option{
			"autoAck":   false,
			"exclusive": false,
			"noLocal":   false,
			"noWait":    false,
		},
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
		for msg := range msgs {
			event := cloudevents.NewEvent()
			err := json.Unmarshal(msg.Body(), &event)
			if err != nil {
				log.Printf("failed to unmarshal event (nacking and not requeueing): %s", err)
				msg.Nack(false, false) // not multiple, do not requeue
				continue
			}

			ctx := cloudevents.ContextWithTarget(context.Background(), d.subscriberURL)
			fmt.Printf("retrying set to: %d", *d.delivery.Retry)
			ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, 50*time.Millisecond, int(*d.delivery.Retry))

			response, result := ceClient.Request(ctx, event)
			if !cloudevents.IsACK(result) {
				log.Printf("failed downstream (nacking and requeueing): %s", result.Error())
				msg.Nack(false, true) // not multiple, do requeue
				continue
			}
			if response != nil {
				ctx = cloudevents.ContextWithTarget(context.Background(), d.brokerIngressURL)
				if result := ceClient.Send(ctx, *response); !cloudevents.IsACK(result) {
					msg.Nack(false, true) // not multiple, do requeue
				}
			}
			msg.Ack(false) // not multiple
		}
	}()

	log.Printf("rabbitmq receiver started, exit with CTRL+C")
	<-forever
}
