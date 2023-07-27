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
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	"go.opencensus.io/trace"
	"go.uber.org/zap"

	"knative.dev/eventing-rabbitmq/pkg/broker/dispatcher"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
)

const (
	ComponentName = "rabbitmq-dispatcher"
)

type Dispatcher struct {
	BrokerIngressURL  string
	SubscriberURL     string
	SubscriberCACerts string
	MaxRetries        int
	BackoffDelay      time.Duration
	Timeout           time.Duration
	BackoffPolicy     eventingduckv1.BackoffPolicyType
	WorkerCount       int
	Reporter          dispatcher.StatsReporter
	DLX               bool
	DLXName           string
}

// ConsumeFromQueue consumes messages from the given message channel and queue.
// When the context is cancelled a context.Canceled error is returned.
func (d *Dispatcher) ConsumeFromQueue(ctx context.Context, conn rabbit.RabbitMQConnectionInterface, channel rabbit.RabbitMQChannelInterface, queueName string) error {
	if channel == nil {
		return amqp.ErrClosed
	}

	msgs, err := channel.Consume(
		queueName, // queue
		"",        // consumer
		false,
		false,
		false,
		false,
		amqp.Table{})
	if err != nil {
		return errors.Wrap(err, "create consumer")
	}

	topts := []cehttp.Option{cehttp.WithIsRetriableFunc(func(statusCode int) bool {
		retry, _ := kncloudevents.SelectiveRetry(ctx, &http.Response{StatusCode: statusCode}, nil)
		return retry
	})}

	if d.Timeout != 0 {
		topts = append(topts, utils.WithTimeout(d.Timeout))
	}

	ceClient, err := client.NewClientHTTP(topts, nil)
	if err != nil {
		return errors.Wrap(err, "create http client")
	}

	logging.FromContext(ctx).Info("rabbitmq receiver started, exit with CTRL+C")
	logging.FromContext(ctx).Infow("starting to process messages", zap.String("queue", queueName), zap.Int("workers", d.WorkerCount))

	wg := &sync.WaitGroup{}
	wg.Add(d.WorkerCount)
	workerQueue := make(chan amqp.Delivery, d.WorkerCount)

	for i := 0; i < d.WorkerCount; i++ {
		go func() {
			defer wg.Done()
			for msg := range workerQueue {
				if d.DLX {
					_ = d.dispatchDLQ(ctx, msg, ceClient)
				} else {
					_ = d.dispatch(ctx, msg, ceClient, channel)
				}
			}
		}()
	}
	// get connections notify channel to watch for any unexpected disconnection
	connNotifyChannel, chNotifyChannel := conn.NotifyClose(make(chan *amqp.Error)), channel.NotifyClose(make(chan *amqp.Error))
	for {
		select {
		case <-ctx.Done():
			logging.FromContext(ctx).Info("context done, stopping message consumers")
			finishConsuming(wg, workerQueue)
			return ctx.Err()
		case <-connNotifyChannel:
			finishConsuming(wg, workerQueue)
			return amqp.ErrClosed
		case <-chNotifyChannel:
			finishConsuming(wg, workerQueue)
			return amqp.ErrClosed
		case msg, ok := <-msgs:
			if !ok {
				finishConsuming(wg, workerQueue)
				return amqp.ErrClosed
			}
			workerQueue <- msg
		}
	}
}

func finishConsuming(wg *sync.WaitGroup, workerQueue chan amqp.Delivery) {
	close(workerQueue)
	wg.Wait()
}

func getStatus(ctx context.Context, result protocol.Result) (int, bool) {
	var retriesResult *cehttp.RetriesResult
	if cloudevents.ResultAs(result, &retriesResult) {
		var httpResult *cehttp.Result
		if cloudevents.ResultAs(retriesResult.Result, &httpResult) {
			retry, _ := kncloudevents.SelectiveRetry(ctx, &http.Response{StatusCode: httpResult.StatusCode}, nil)
			return httpResult.StatusCode, !retry
		}
		logging.FromContext(ctx).Warnf("invalid result type, not HTTP Result: %v", retriesResult.Result)
		return -1, false
	}

	logging.FromContext(ctx).Warnf("invalid result type, not RetriesResult")
	return -1, false
}

func (d *Dispatcher) dispatch(ctx context.Context, msg amqp.Delivery, ceClient cloudevents.Client, channel rabbit.RabbitMQChannelInterface) error {
	start := time.Now()
	dlqExchange := d.DLXName

	ctx, span := readSpan(ctx, msg)
	defer span.End()

	msgBinding := rabbit.NewMessageFromDelivery(ComponentName, "", "", &msg)
	event, err := binding.ToEvent(cloudevents.WithEncodingBinary(ctx), msgBinding)
	if err != nil {
		logging.FromContext(ctx).Warn("failed parsing event: ", err)
		if err = msg.Ack(false); err != nil {
			logging.FromContext(ctx).Warn("failed to Ack event: ", err)
		}

		return fmt.Errorf("failed parsing event: %s", err)
	}

	if span.IsRecordingEvents() {
		span.AddAttributes(client.EventTraceAttributes(event)...)
	}

	ctx = cloudevents.ContextWithTarget(ctx, d.SubscriberURL)
	if d.BackoffPolicy == eventingduckv1.BackoffPolicyLinear {
		ctx = cloudevents.ContextWithRetriesLinearBackoff(ctx, d.BackoffDelay, d.MaxRetries)
	} else {
		ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, d.BackoffDelay, d.MaxRetries)
	}

	response, result := ceClient.Request(ctx, *event)
	statusCode, isSuccess := getStatus(ctx, result)
	if statusCode != -1 {
		args := &dispatcher.ReportArgs{EventType: event.Type()}
		if err = d.Reporter.ReportEventCount(args, statusCode); err != nil {
			logging.FromContext(ctx).Errorf("something happened: %v", err)
		}
	}

	if !isSuccess {
		logging.FromContext(ctx).Warnf("failed to deliver to %q", d.SubscriberURL)

		// We need to ack the original message.
		if err = msg.Ack(false); err != nil {
			logging.FromContext(ctx).Warn("failed to Ack event: ", err)
		}

		// Add headers as described here: https://knative.dev/docs/eventing/event-delivery/#configuring-channel-event-delivery
		event.SetExtension("knativeerrordest", d.SubscriberURL)
		event.SetExtension("knativeerrorcode", statusCode)

		// Queue the event into DLQ with the correct headers.
		if err = sendToRabbitMQ(channel, dlqExchange, span, event); err != nil {
			logging.FromContext(ctx).Warn("failed to send event: ", err)
		}
		return fmt.Errorf("failed to deliver to %q", d.SubscriberURL)
	} else if response != nil {
		logging.FromContext(ctx).Infof("sending an event: %+v", response)
		ctx = cloudevents.ContextWithTarget(ctx, d.BrokerIngressURL)
		result = ceClient.Send(ctx, *response)
		_, isSuccess = getStatus(ctx, result)
		if !isSuccess {
			logging.FromContext(ctx).Warnf("failed to deliver to %q", d.BrokerIngressURL)

			// We need to ack the original message.
			if err = msg.Ack(false); err != nil {
				logging.FromContext(ctx).Warn("failed to Ack event: ", err)
			}

			// Add headers as described here: https://knative.dev/docs/eventing/event-delivery/#configuring-channel-event-delivery
			event.SetExtension("knativeerrordest", d.SubscriberURL)
			event.SetExtension("knativeerrorcode", statusCode)
			event.SetExtension("knativeerrordata", result)

			// Queue the event into DLQ with the correct headers.
			if err = sendToRabbitMQ(channel, dlqExchange, span, event); err != nil {
				logging.FromContext(ctx).Warn("failed to send event: ", err)
			}
			return fmt.Errorf("failed to deliver to %q", d.BrokerIngressURL)
		}
	}

	if err = msg.Ack(false); err != nil {
		logging.FromContext(ctx).Warn("failed to Ack event: ", err)
	}
	if statusCode != -1 {
		args := &dispatcher.ReportArgs{EventType: event.Type()}
		dispatchTime := time.Since(start)
		_ = d.Reporter.ReportEventDispatchTime(args, statusCode, dispatchTime)
	}
	return nil
}

// Defaulting to Ack as this always hits the DLQ.
func (d *Dispatcher) dispatchDLQ(ctx context.Context, msg amqp.Delivery, ceClient cloudevents.Client) error {
	start := time.Now()
	msgBinding := rabbit.NewMessageFromDelivery(ComponentName, "", "", &msg)
	event, err := binding.ToEvent(cloudevents.WithEncodingBinary(ctx), msgBinding)
	if err != nil {
		logging.FromContext(ctx).Warn("failed creating event from delivery, err: ", err)
		if err = msg.Ack(false); err != nil {
			logging.FromContext(ctx).Warn("failed to Ack event: ", err)
		}
		return fmt.Errorf("failed creating event from delivery, err: %s", err)
	}

	ctx, span := readSpan(ctx, msg)
	defer span.End()
	if span.IsRecordingEvents() {
		span.AddAttributes(client.EventTraceAttributes(event)...)
	}

	ctx = cloudevents.ContextWithTarget(ctx, d.SubscriberURL)
	if d.BackoffPolicy == eventingduckv1.BackoffPolicyLinear {
		ctx = cloudevents.ContextWithRetriesLinearBackoff(ctx, d.BackoffDelay, d.MaxRetries)
	} else {
		ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, d.BackoffDelay, d.MaxRetries)
	}

	response, result := ceClient.Request(ctx, *event)
	statusCode, isSuccess := getStatus(ctx, result)
	if statusCode != -1 {
		args := &dispatcher.ReportArgs{EventType: event.Type()}
		if err = d.Reporter.ReportEventCount(args, statusCode); err != nil {
			logging.FromContext(ctx).Errorf("something happened: %v", err)
		}
	}

	if !isSuccess {
		logging.FromContext(ctx).Warnf("failed to deliver to %q %s", d.SubscriberURL, msg)
		if err = msg.Ack(false); err != nil {
			logging.FromContext(ctx).Warn("failed to Ack event: ", err)
		}
		return fmt.Errorf("failed to deliver to %q", d.SubscriberURL)
	} else if response != nil {
		logging.FromContext(ctx).Infof("Sending an event: %+v", response)
		ctx = cloudevents.ContextWithTarget(ctx, d.BrokerIngressURL)
		result = ceClient.Send(ctx, *response)
		_, isSuccess = getStatus(ctx, result)
		if !isSuccess {
			logging.FromContext(ctx).Warnf("failed to deliver to %q", d.BrokerIngressURL)
			if err = msg.Ack(false); err != nil {
				logging.FromContext(ctx).Warn("failed to Ack event: ", err)
			}
			return fmt.Errorf("failed to deliver to %q", d.BrokerIngressURL)
		}
	}

	err = msg.Ack(false)
	if err != nil {
		logging.FromContext(ctx).Warn("failed to Ack event: ", err)
	}
	if statusCode != -1 {
		args := &dispatcher.ReportArgs{EventType: event.Type()}
		dispatchTime := time.Since(start)
		_ = d.Reporter.ReportEventDispatchTime(args, statusCode, dispatchTime)
	}
	return nil
}

func sendToRabbitMQ(channel rabbit.RabbitMQChannelInterface, exchangeName string, span *trace.Span, event *cloudevents.Event) error {
	// no dlq defined in the trigger nor the broker, return
	if exchangeName == "" {
		return nil
	}

	tp, ts := (&tracecontext.HTTPFormat{}).SpanContextToHeaders(span.SpanContext())
	dc, err := channel.PublishWithDeferredConfirm(
		exchangeName,
		"",    // routing key
		false, // mandatory
		false, // immediate
		*rabbit.CloudEventToRabbitMQMessage(event, tp, ts))
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	if ack := dc.Wait(); !ack {
		return errors.New("failed to publish message: nacked")
	}
	return nil
}

func readSpan(ctx context.Context, msg amqp.Delivery) (context.Context, *trace.Span) {
	traceparent, ok := msg.Headers["traceparent"].(string)
	if !ok {
		return ctx, nil
	}
	tracestate, ok := msg.Headers["tracestate"].(string)
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
