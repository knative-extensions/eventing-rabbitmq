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

	"github.com/cloudevents/sdk-go/observability/opentelemetry/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability"
	"knative.dev/pkg/logging"
)

const (
	ComponentName     = "rabbitmq-dispatcher"
	traceParentHeader = "traceparent"
	traceStateHeader  = "tracestate"
)

var (
	propagationContext = propagation.TraceContext{}
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
	DLX               bool
	DLXName           string
	DispatchDuration  metric.Float64Histogram
	Tracer            trace.Tracer
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
	connNotifyChannel, chNotifyChannel := conn.NotifyClose(make(chan *amqp.Error, 1)), channel.NotifyClose(make(chan *amqp.Error, 1))
	for {
		select {
		case <-ctx.Done():
			logging.FromContext(ctx).Info("context done, stopping message consumers")
			finishConsuming(wg, workerQueue)
			return ctx.Err()
		case err = <-connNotifyChannel:
			finishConsuming(wg, workerQueue)
			// No error will be available in case of a graceful connection close
			if err == nil {
				err = amqp.ErrClosed
			}
			return err
		case err = <-chNotifyChannel:
			finishConsuming(wg, workerQueue)
			// No error will be available in case of a graceful connection close
			if err == nil {
				err = amqp.ErrClosed
			}
			return err
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
	dlqExchange := d.DLXName

	ctx = observability.WithMessagingLabels(ctx, d.SubscriberURL, "send")
	start := time.Now()

	var event *cloudevents.Event
	var err error

	ctx, span := readSpan(ctx, msg, d.Tracer)
	defer func() {
		if span == nil {
			return
		}

		if span.IsRecording() {
			if event != nil {
				// add event labels here so they only affect the trace, not the metrics
				ctx = observability.WithEventLabels(ctx, event)
			}
			labeler, _ := otelhttp.LabelerFromContext(ctx)
			span.SetAttributes(labeler.Get()...)
		}
		span.End()
	}()

	msgBinding := rabbit.NewMessageFromDelivery(ComponentName, "", "", &msg)
	event, err = binding.ToEvent(cloudevents.WithEncodingBinary(ctx), msgBinding)
	if err != nil {
		logging.FromContext(ctx).Warn("failed parsing event: ", err)
		if err = msg.Ack(false); err != nil {
			logging.FromContext(ctx).Warn("failed to Ack event: ", err)
		}

		return fmt.Errorf("failed parsing event: %s", err)
	}

	ctx = observability.WithMinimalEventLabels(ctx, event)

	// defer the reporting here, as we now know that we actually have an event, so "dispatching" makes sense
	defer func() {
		dispatchSeconds := time.Since(start).Seconds()
		labeler, _ := otelhttp.LabelerFromContext(ctx)
		d.DispatchDuration.Record(ctx, dispatchSeconds, metric.WithAttributes(labeler.Get()...))
	}()

	ctx = cloudevents.ContextWithTarget(ctx, d.SubscriberURL)
	if d.BackoffPolicy == eventingduckv1.BackoffPolicyLinear {
		ctx = cloudevents.ContextWithRetriesLinearBackoff(ctx, d.BackoffDelay, d.MaxRetries)
	} else {
		ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, d.BackoffDelay, d.MaxRetries)
	}

	response, result := ceClient.Request(ctx, *event)
	statusCode, isSuccess := getStatus(ctx, result)

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
		if err = sendToRabbitMQ(ctx, channel, dlqExchange, event); err != nil {
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
			if err = sendToRabbitMQ(ctx, channel, dlqExchange, event); err != nil {
				logging.FromContext(ctx).Warn("failed to send event: ", err)
			}
			return fmt.Errorf("failed to deliver to %q", d.BrokerIngressURL)
		}
	}

	if err = msg.Ack(false); err != nil {
		logging.FromContext(ctx).Warn("failed to Ack event: ", err)
	}
	return nil
}

// Defaulting to Ack as this always hits the DLQ.
func (d *Dispatcher) dispatchDLQ(ctx context.Context, msg amqp.Delivery, ceClient cloudevents.Client) error {
	start := time.Now()

	ctx = observability.WithMessagingLabels(ctx, d.SubscriberURL, "send")

	msgBinding := rabbit.NewMessageFromDelivery(ComponentName, "", "", &msg)
	event, err := binding.ToEvent(cloudevents.WithEncodingBinary(ctx), msgBinding)
	if err != nil {
		logging.FromContext(ctx).Warn("failed creating event from delivery, err: ", err)
		if err = msg.Ack(false); err != nil {
			logging.FromContext(ctx).Warn("failed to Ack event: ", err)
		}
		return fmt.Errorf("failed creating event from delivery, err: %s", err)
	}

	ctx = observability.WithMinimalEventLabels(ctx, event)

	// defer the reporting here, as we now know that we actually have an event, so "dispatching" makes sense
	defer func() {
		dispatchSeconds := time.Since(start).Seconds()
		labeler, _ := otelhttp.LabelerFromContext(ctx)
		d.DispatchDuration.Record(ctx, dispatchSeconds, metric.WithAttributes(labeler.Get()...))
	}()

	ctx, span := readSpan(ctx, msg, d.Tracer)
	defer func() {
		if span == nil {
			return
		}

		if span.IsRecording() {
			// add event labels here so they only affect the trace, not the metrics
			ctx = observability.WithEventLabels(ctx, event)
			labeler, _ := otelhttp.LabelerFromContext(ctx)
			span.SetAttributes(labeler.Get()...)
		}
		span.End()
	}()

	ctx = cloudevents.ContextWithTarget(ctx, d.SubscriberURL)
	if d.BackoffPolicy == eventingduckv1.BackoffPolicyLinear {
		ctx = cloudevents.ContextWithRetriesLinearBackoff(ctx, d.BackoffDelay, d.MaxRetries)
	} else {
		ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, d.BackoffDelay, d.MaxRetries)
	}

	response, result := ceClient.Request(ctx, *event)
	_, isSuccess := getStatus(ctx, result)

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
	return nil
}

func sendToRabbitMQ(ctx context.Context, channel rabbit.RabbitMQChannelInterface, exchangeName string, event *cloudevents.Event) error {
	// no dlq defined in the trigger nor the broker, return
	if exchangeName == "" {
		return nil
	}

	tp, ts := extractSpanHeaders(ctx)
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

func readSpan(ctx context.Context, msg amqp.Delivery, tracer trace.Tracer) (context.Context, trace.Span) {
	traceParent, ok := msg.Headers["traceparent"].(string)
	if !ok {
		return ctx, nil
	}
	traceState, ok := msg.Headers["tracestate"].(string)
	if !ok {
		return ctx, nil
	}
	ctx = injectSpanContext(ctx, traceParent, traceState)
	if sc := trace.SpanContextFromContext(ctx); !sc.IsValid() {
		return ctx, nil
	}

	return tracer.Start(ctx, ComponentName, trace.WithSpanKind(trace.SpanKindServer))
}

func extractSpanHeaders(ctx context.Context) (traceParent, traceState string) {
	headerCarrier := propagation.HeaderCarrier{}
	propagationContext.Inject(ctx, headerCarrier)
	return headerCarrier.Get(traceParentHeader), headerCarrier.Get(traceStateHeader)
}

func injectSpanContext(ctx context.Context, traceParent, traceState string) context.Context {
	headerCarrier := propagation.HeaderCarrier{}
	headerCarrier.Set(traceParentHeader, traceParent)
	headerCarrier.Set(traceStateHeader, traceState)

	ctx = propagationContext.Extract(ctx, headerCarrier)

	return ctx
}
