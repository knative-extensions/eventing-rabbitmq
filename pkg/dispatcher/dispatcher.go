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
	"time"

	"github.com/NeowayLabs/wabbit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
)

type Dispatcher struct {
	queueName        string
	brokerURL        string
	brokerIngressURL string
	subscriberURL    string
	dlqDispatcher    bool
	delivery         *eventingduckv1.DeliverySpec
	dialerFunc       dialer.DialerFunc
	fakeChannel      *wabbit.Channel
}

func NewDispatcher(queueName, brokerURL, brokerIngressURL, subscriberURL string, dlqDispatcher bool, delivery *eventingduckv1.DeliverySpec) *Dispatcher {
	return &Dispatcher{
		queueName:        queueName,
		brokerURL:        brokerURL,
		brokerIngressURL: brokerIngressURL,
		subscriberURL:    subscriberURL,
		dlqDispatcher:    dlqDispatcher,
		delivery:         delivery,
		dialerFunc:       dialer.RealDialer,
	}

}

func (d *Dispatcher) SetDialerFunc(dialerFunc dialer.DialerFunc) {
	d.dialerFunc = dialerFunc
}

func (d *Dispatcher) SetFakeChannel(fakeChannel *wabbit.Channel) {
	d.fakeChannel = fakeChannel
}

func (d *Dispatcher) Start(ctx context.Context) {
	logging.FromContext(ctx).Infow("Starting dispatcher for: ", zap.String("Queue", d.queueName))
	conn, err := d.dialerFunc(d.brokerURL)
	if err != nil {
		logging.FromContext(ctx).Fatalf("failed to connect to RabbitMQ: %s", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if d.fakeChannel != nil {
		channel = *d.fakeChannel
	}
	if err != nil {
		logging.FromContext(ctx).Fatalf("failed to open a channel: %s", err)
	}
	defer channel.Close()

	err = channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		logging.FromContext(ctx).Fatalf("failed to create QoS: %s", err)
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
		logging.FromContext(ctx).Fatalf("failed to create consumer: %s", err)
	}

	forever := make(chan bool)

	sender, err := kncloudevents.NewHttpMessageSender(nil, d.subscriberURL)
	if err != nil {
		logging.FromContext(ctx).Fatal("failed to create http client")
	}
	logging.FromContext(ctx).Infow("Created: ", zap.Any("sender", sender))

	ceClient, err := cloudevents.NewDefaultClient()
	if err != nil {
		logging.FromContext(ctx).Fatal("failed to create http client")
	}

	go func() {
		for msg := range msgs {
			logging.FromContext(ctx).Infof("Got a message")
			event := cloudevents.NewEvent()
			err := json.Unmarshal(msg.Body(), &event)
			if err != nil {
				logging.FromContext(ctx).Infof("failed to unmarshal event (nacking and not requeueing): %s", err)
				msg.Nack(false, false) // not multiple, do not requeue
				continue
			}

			ctx = cloudevents.ContextWithTarget(ctx, d.subscriberURL)

			// Whether we should requeue, or if DLQ has been specified, then don't.
			requeue := true
			if d.delivery != nil {
				if d.delivery.Retry != nil || d.delivery.BackoffPolicy != nil || d.delivery.BackoffDelay != nil {
					requeue = false
					var retry int
					retry = 1
					if d.delivery.Retry != nil {
						retry = int(*d.delivery.Retry)
					}
					var backoffDelay time.Duration
					backoffDelay = 50 * time.Millisecond
					if d.delivery.BackoffDelay != nil {
						// DO NOT SUBMIT: How to parse this???
						// backoffDelay = time.Duration.Parse(*d.delivery.BackoffDelay)
					}

					// Defaults to exponential unless explicitly set to linear.
					if d.delivery.BackoffPolicy == nil || *d.delivery.BackoffPolicy != eventingduckv1.BackoffPolicyLinear {
						ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, backoffDelay, retry)
					} else {
						ctx = cloudevents.ContextWithRetriesLinearBackoff(ctx, backoffDelay, retry)
					}
				}
			}
			// Do not allow requeueing of the failed events.
			if d.dlqDispatcher {
				requeue = false
			}

			response, result := ceClient.Request(ctx, event)
			if !isSuccess(ctx, result) {
				logging.FromContext(ctx).Warnf("Failed to deliver to %q", d.subscriberURL)
				msg.Nack(false, requeue) // not multiple, do requeue
				continue
			}

			// TODO(vaikas): Should we allow maybe "fixed" events to be delivered back to the broker?
			// Seems reasonable, but for now, don't allow it.
			if !d.dlqDispatcher && response != nil {
				ctx = cloudevents.ContextWithTarget(ctx, d.brokerIngressURL)
				backoffDelay := 50 * time.Millisecond
				cloudevents.ContextWithRetriesExponentialBackoff(ctx, backoffDelay, 1)
				result := ceClient.Send(ctx, *response)
				if !isSuccess(ctx, result) {
					logging.FromContext(ctx).Warnf("Failed to deliver to %q", d.brokerURL)
					msg.Nack(false, requeue) // not multiple, do requeue
					continue
				}
			}
			msg.Ack(false) // not multiple
		}
	}()

	logging.FromContext(ctx).Infof("rabbitmq receiver started, exit with CTRL+C")
	<-forever
}

func isSuccess(ctx context.Context, result protocol.Result) bool {
	var retriesResult *cehttp.RetriesResult
	if cloudevents.ResultAs(result, &retriesResult) {
		var httpResult *cehttp.Result
		if cloudevents.ResultAs(retriesResult.Result, &httpResult) {
			if httpResult.StatusCode > 199 && httpResult.StatusCode < 300 {
				logging.FromContext(ctx).Infof("Got response code: %d", httpResult.StatusCode)
				return true
			}
		} else {
			logging.FromContext(ctx).Warnf("Invalid result type, not HTTP Result")
		}
	} else {
		logging.FromContext(ctx).Warnf("Invalid result type, not RetriesResult")
	}
	return false
}
