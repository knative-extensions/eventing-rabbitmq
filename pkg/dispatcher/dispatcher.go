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
	"net/http"
	"sync"
	"time"

	"github.com/NeowayLabs/wabbit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/pkg/errors"
	amqperr "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
)

const (
	ackMultiple = false // send ack/nack for multiple messages
)

type Dispatcher struct {
	BrokerIngressURL string
	SubscriberURL    string

	// Upon failure to deliver to sink, should the RabbitMQ messages be requeued.
	// For example, if the DeadLetterSink has been configured, then do not Requeue.
	Requeue bool

	MaxRetries    int
	BackoffDelay  time.Duration
	BackoffPolicy eventingduckv1.BackoffPolicyType
	WorkerCount   int
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
	logging.FromContext(ctx).Infow("Starting to process messages", zap.String("queue", queueName), zap.Int("workers", d.WorkerCount))

	wg := &sync.WaitGroup{}
	wg.Add(d.WorkerCount)
	workerQueue := make(chan wabbit.Delivery, d.WorkerCount)

	for i := 0; i < d.WorkerCount; i++ {
		go d.dispatch(ctx, wg, workerQueue, ceClient)
	}
	for {
		select {
		case <-ctx.Done():
			logging.FromContext(ctx).Info("context done, stopping message consumers")
			close(workerQueue)
			wg.Wait()
			return ctx.Err()

		case msg, ok := <-msgs:
			if !ok {
				logging.FromContext(ctx).Warn("message channel closed, stopping message consumers")
				close(workerQueue)
				wg.Wait()
				return amqperr.ErrClosed
			}
			workerQueue <- msg
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
			retry, _ := kncloudevents.SelectiveRetry(ctx, &http.Response{StatusCode: httpResult.StatusCode}, nil)
			return !retry
		}
		logging.FromContext(ctx).Warnf("Invalid result type, not HTTP Result: %v", retriesResult.Result)
		return false
	}

	logging.FromContext(ctx).Warnf("Invalid result type, not RetriesResult")
	return false
}

func (d *Dispatcher) dispatch(ctx context.Context, wg *sync.WaitGroup, queue <-chan wabbit.Delivery, ceClient cloudevents.Client) {
	defer wg.Done()

	for msg := range queue {
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
		ctx = cloudevents.ContextWithTarget(ctx, d.SubscriberURL)

		if d.BackoffPolicy == eventingduckv1.BackoffPolicyLinear {
			ctx = cloudevents.ContextWithRetriesLinearBackoff(ctx, d.BackoffDelay, d.MaxRetries)
		} else {
			ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, d.BackoffDelay, d.MaxRetries)
		}

		response, result := ceClient.Request(ctx, event)
		if !isSuccess(ctx, result) {
			logging.FromContext(ctx).Warnf("Failed to deliver to %q requeue: %v", d.SubscriberURL, d.Requeue)
			err = msg.Nack(ackMultiple, d.Requeue)
			if err != nil {
				logging.FromContext(ctx).Warn("failed to NACK event: ", err)
			}
			continue
		}

		logging.FromContext(ctx).Debugf("Got Response: %+v", response)
		if response != nil {
			logging.FromContext(ctx).Infof("Sending an event: %+v", response)
			ctx = cloudevents.ContextWithTarget(ctx, d.BrokerIngressURL)
			cloudevents.ContextWithRetriesExponentialBackoff(ctx, d.BackoffDelay, d.MaxRetries)
			result := ceClient.Send(ctx, *response)
			if !isSuccess(ctx, result) {
				logging.FromContext(ctx).Warnf("Failed to deliver to %q requeue: %v", d.BrokerIngressURL, d.Requeue)
				err = msg.Nack(ackMultiple, d.Requeue) // not multiple
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
