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
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
)

type Dispatcher struct {
	brokerIngressURL string
	subscriberURL    string

	// Upon failure to deliver to sink, should the RabbitMQ messages be requeued.
	// For example, if the DeadLetterSink has been configured, then do not requeue.
	requeue bool

	maxRetries    int
	backoffDelay  time.Duration
	backoffPolicy eventingduckv1.BackoffPolicyType
}

func NewDispatcher(brokerIngressURL, subscriberURL string, requeue bool, maxRetries int, backoffDelay time.Duration, backoffPolicy eventingduckv1.BackoffPolicyType) *Dispatcher {
	return &Dispatcher{
		brokerIngressURL: brokerIngressURL,
		subscriberURL:    subscriberURL,
		requeue:          requeue,
		maxRetries:       maxRetries,
		backoffDelay:     backoffDelay,
		backoffPolicy:    backoffPolicy,
	}

}

func (d *Dispatcher) ConsumeFromQueue(ctx context.Context, channel wabbit.Channel, queueName string) {
	logging.FromContext(ctx).Infow("Starting to process message for: ", zap.String("Queue", queueName))
	msgs, err := channel.Consume(
		queueName, // queue
		"",        // consumer
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

			if d.backoffPolicy == eventingduckv1.BackoffPolicyLinear {
				ctx = cloudevents.ContextWithRetriesLinearBackoff(ctx, d.backoffDelay, d.maxRetries)
			} else {
				ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, d.backoffDelay, d.maxRetries)
			}

			response, result := ceClient.Request(ctx, event)
			if !isSuccess(ctx, result) {
				logging.FromContext(ctx).Warnf("Failed to deliver to %q requeue: %v", d.subscriberURL, d.requeue)
				msg.Nack(false, d.requeue) // not multiple
				continue
			}

			if response != nil {
				ctx = cloudevents.ContextWithTarget(ctx, d.brokerIngressURL)
				backoffDelay := 50 * time.Millisecond
				// Use the retries so we can just parse out the results in a common way.
				cloudevents.ContextWithRetriesExponentialBackoff(ctx, backoffDelay, 1)
				result := ceClient.Send(ctx, *response)
				if !isSuccess(ctx, result) {
					logging.FromContext(ctx).Warnf("Failed to deliver to %q requeue: %v", d.brokerIngressURL, d.requeue)
					msg.Nack(false, d.requeue) // not multiple
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
