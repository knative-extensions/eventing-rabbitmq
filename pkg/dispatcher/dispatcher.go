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
	"github.com/pkg/errors"
	amqperr "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
)

const (
	ackMultiple = false // send ack/nack for multiple messages
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

// ConsumeFromQueue consumes messages from the given message channel and queue.
// When the context is cancelled a context.Canceled error is returned.
func (d *Dispatcher) ConsumeFromQueue(ctx context.Context, channel wabbit.Channel, queueName string) error {
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
		return errors.Wrap(err, "create consumer")
	}

	ceClient, err := cloudevents.NewClientHTTP(cehttp.WithIsRetriableFunc(isRetriableFunc))
	if err != nil {
		return errors.Wrap(err, "create http client")
	}

	logging.FromContext(ctx).Info("rabbitmq receiver started, exit with CTRL+C")
	logging.FromContext(ctx).Infow("Starting to process messages", zap.String("queue", queueName))

	for {
		select {
		case <-ctx.Done():
			logging.FromContext(ctx).Info("context done, stopping message consumer")
			return ctx.Err()

		case msg, ok := <-msgs:
			if !ok {
				logging.FromContext(ctx).Warn("message channel closed, stopping message consumer")
				return amqperr.ErrClosed
			}

			event := cloudevents.NewEvent()
			err := json.Unmarshal(msg.Body(), &event)
			if err != nil {
				logging.FromContext(ctx).Warn("failed to unmarshal event (NACK-ing and not re-queueing): ", err)
				err = msg.Nack(ackMultiple, false) // do not requeue
				if err != nil {
					logging.FromContext(ctx).Warn("failed to NACK event: ", err)
				}
				continue
			}

			logging.FromContext(ctx).Debugf("Got event as: %+v", event)
			ctx = cloudevents.ContextWithTarget(ctx, d.subscriberURL)

			// Our dispatcher uses Retries, but cloudevents is the max total tries. So we need
			// to adjust to initial + retries.
			// TODO: What happens if I specify 0 to cloudevents. Does it not even retry.
			retryCount := d.maxRetries
			if d.backoffPolicy == eventingduckv1.BackoffPolicyLinear {
				ctx = cloudevents.ContextWithRetriesLinearBackoff(ctx, d.backoffDelay, retryCount)
			} else {
				ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, d.backoffDelay, retryCount)
			}

			response, result := ceClient.Request(ctx, event)
			if !isSuccess(ctx, result) {
				logging.FromContext(ctx).Warnf("Failed to deliver to %q requeue: %v", d.subscriberURL, d.requeue)
				err = msg.Nack(ackMultiple, d.requeue)
				if err != nil {
					logging.FromContext(ctx).Warn("failed to NACK event: ", err)
				}
				continue
			}

			logging.FromContext(ctx).Debugf("Got Response: %+v", response)
			if response != nil {
				logging.FromContext(ctx).Infof("Sending an event: %+v", response)
				ctx = cloudevents.ContextWithTarget(ctx, d.brokerIngressURL)
				backoffDelay := 50 * time.Millisecond
				// Use the retries so we can just parse out the results in a common way.
				cloudevents.ContextWithRetriesExponentialBackoff(ctx, backoffDelay, 1)
				result := ceClient.Send(ctx, *response)
				if !isSuccess(ctx, result) {
					logging.FromContext(ctx).Warnf("Failed to deliver to %q requeue: %v", d.brokerIngressURL, d.requeue)
					err = msg.Nack(ackMultiple, d.requeue) // not multiple
					if err != nil {
						logging.FromContext(ctx).Warn("failed to NACK event: ", err)
					}
					continue
				}
			}

			err = msg.Ack(ackMultiple)
			if err != nil {
				logging.FromContext(ctx).Warn("failed to ACK event: ", err)
			}
		}
	}
}

func isRetriableFunc(sc int) bool {
	return sc < 200 || sc >= 300
}

func isSuccess(ctx context.Context, result protocol.Result) bool {
	var retriesResult *cehttp.RetriesResult
	if cloudevents.ResultAs(result, &retriesResult) {
		var httpResult *cehttp.Result
		if cloudevents.ResultAs(retriesResult.Result, &httpResult) {
			if httpResult.StatusCode > 199 && httpResult.StatusCode < 300 {
				return true
			} else {
				return false
			}
		}
		logging.FromContext(ctx).Warnf("Invalid result type, not HTTP Result")
		return false
	}

	logging.FromContext(ctx).Warnf("Invalid result type, not RetriesResult")
	return false
}
