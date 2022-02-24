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
	"github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/pkg/errors"
	amqperr "github.com/rabbitmq/amqp091-go"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
)

const (
	ComponentName = "rabbitmq-dispatcher"
)

type Dispatcher struct {
	BrokerIngressURL string
	SubscriberURL    string
	MaxRetries       int
	BackoffDelay     time.Duration
	BackoffPolicy    eventingduckv1.BackoffPolicyType
	WorkerCount      int
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

	ceClient, err := client.NewClientHTTP([]cehttp.Option{cehttp.WithIsRetriableFunc(func(statusCode int) bool {
		retry, _ := kncloudevents.SelectiveRetry(ctx, &http.Response{StatusCode: statusCode}, nil)
		return retry
	})}, nil)
	if err != nil {
		return errors.Wrap(err, "create http client")
	}

	logging.FromContext(ctx).Info("rabbitmq receiver started, exit with CTRL+C")
	logging.FromContext(ctx).Infow("Starting to process messages", zap.String("queue", queueName), zap.Int("workers", d.WorkerCount))

	wg := &sync.WaitGroup{}
	wg.Add(d.WorkerCount)
	workerQueue := make(chan wabbit.Delivery, d.WorkerCount)

	for i := 0; i < d.WorkerCount; i++ {
		go func() {
			defer wg.Done()
			for msg := range workerQueue {
				d.dispatch(ctx, msg, ceClient)
			}
		}()
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

func (d *Dispatcher) dispatch(ctx context.Context, msg wabbit.Delivery, ceClient cloudevents.Client) {
	event := cloudevents.NewEvent()
	err := json.Unmarshal(msg.Body(), &event)
	if err != nil {
		logging.FromContext(ctx).Warn("failed to unmarshal event (NACK-ing and not re-queueing): ", err)
		err = msg.Nack(false, false)
		if err != nil {
			logging.FromContext(ctx).Warn("failed to NACK event: ", err)
		}
		return
	}

	ctx, span := readSpan(ctx, msg)
	defer span.End()
	if span.IsRecordingEvents() {
		span.AddAttributes(client.EventTraceAttributes(&event)...)
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
		logging.FromContext(ctx).Warnf("Failed to deliver to %q", d.SubscriberURL)
		err := msg.Nack(false, false); if err != nil {
			logging.FromContext(ctx).Warn("failed to NACK event: ", err)
		}
		return
	}

	logging.FromContext(ctx).Debugf("Got Response: %+v", response)
	if response != nil {
		logging.FromContext(ctx).Infof("Sending an event: %+v", response)
		ctx = cloudevents.ContextWithTarget(ctx, d.BrokerIngressURL)
		result := ceClient.Send(ctx, *response)
		if !isSuccess(ctx, result) {
			logging.FromContext(ctx).Warnf("Failed to deliver to %q", d.BrokerIngressURL)
			err = msg.Nack(false, false) // not multiple
			if err != nil {
				logging.FromContext(ctx).Warn("failed to NACK event: ", err)
			}
			return
		}
	}

	err = msg.Ack(false)
	if err != nil {
		logging.FromContext(ctx).Warn("failed to ACK event: ", err)
	}
}

func readSpan(ctx context.Context, msg wabbit.Delivery) (context.Context, *trace.Span) {
	traceparent, ok := msg.Headers()["traceparent"].(string)
	if !ok {
		return ctx, nil
	}
	tracestate, ok := msg.Headers()["tracestate"].(string)
	if !ok {
		return ctx, nil
	}
	sc, ok := (&tracecontext.HTTPFormat{}).SpanContextFromHeaders(traceparent, tracestate)
	var span *trace.Span
	if ok {
		ctx, span = trace.StartSpanWithRemoteParent(ctx, ComponentName, sc,
			trace.WithSpanKind(trace.SpanKindServer))
	}
	return ctx, span
}
