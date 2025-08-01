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
	"errors"
	"fmt"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

const (
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 1000
	component                        = "rabbitmq-ingress"

	// noDuration signals that the dispatch step hasn't started
	noDuration = -1

	scopeName = "knative.dev/eventing-rabbitmq/cmd/ingress"
)

var (
	latencyBounds      = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
	propagationContext = propagation.TraceContext{}
)

type envConfig struct {
	utils.EnvConfig

	Port          int    `envconfig:"PORT" default:"8080"`
	BrokerURL     string `envconfig:"BROKER_URL" required:"true"`
	ExchangeName  string `envconfig:"EXCHANGE_NAME" required:"true"`
	RabbitMQVhost string `envconfig:"RABBITMQ_VHOST" required:"false"`

	rmqHelper rabbit.RabbitMQConnectionsHandlerInterface

	ContainerName   string `envconfig:"CONTAINER_NAME" default:"ingress"`
	PodName         string `envconfig:"POD_NAME" default:"rabbitmq-broker-ingress"`
	BrokerName      string `envconfig:"BROKER_NAME"`
	BrokerNamespace string `envconfig:"BROKER_NAMESPACE"`

	tracer           trace.Tracer
	dispatchDuration metric.Float64Histogram
}

func main() {
	ctx := signals.NewContext()
	var err error

	var env envConfig
	if err = envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var: ", err)
	}

	env.SetComponent(component)
	logger := env.GetLogger()
	ctx = logging.WithLogger(ctx, logger)

	err = env.SetupObservability(ctx)
	if err != nil {
		logger.Errorw("Failed setting up observability", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		env.ShutdownObservability(ctx)
	}()

	env.rmqHelper = rabbit.NewRabbitMQConnectionHandler(5, 1000, logger)
	env.rmqHelper.Setup(ctx, rabbit.VHostHandler(env.BrokerURL, env.RabbitMQVhost), rabbit.ChannelConfirm, rabbit.DialWrapper)

	env.tracer = otel.GetTracerProvider().Tracer(scopeName)

	meter := otel.GetMeterProvider().Meter(scopeName)
	env.dispatchDuration, err = meter.Float64Histogram(
		"kn.eventing.dispatch.duration",
		metric.WithDescription("The time to dispatch the event"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBounds...),
	)
	if err != nil {
		logger.Fatalf("failed to create dispatch metric: %s", err.Error())
	}

	connectionArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}
	kncloudevents.ConfigureConnectionArgs(&connectionArgs)
	receiver := kncloudevents.NewHTTPEventReceiver(env.Port)
	if err = receiver.StartListen(ctx, &env); err != nil {
		logger.Fatalf("failed to start listen, %v", err)
	}
}

func (env *envConfig) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	logger := env.GetLogger()
	// validate request method
	if request.Method != http.MethodPost {
		logger.Warn("unexpected request method", zap.String("method", request.Method))
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// validate request URI
	if request.RequestURI != "/" {
		logger.Error("unexpected incoming request uri")
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	ctx := request.Context()
	message := cehttp.NewMessageFromHttpRequest(request)
	defer message.Finish(nil)

	event, err := binding.ToEvent(ctx, message)
	if err != nil {
		logger.Warnw("failed to extract event from request", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// run validation for the extracted event
	validationErr := event.Validate()
	if validationErr != nil {
		logger.Warnw("failed to validate extracted event", zap.Error(validationErr))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx = observability.WithBrokerLabels(ctx, types.NamespacedName{Name: env.BrokerName, Namespace: env.BrokerNamespace})

	span := trace.SpanFromContext(ctx)
	defer func() {
		if span.IsRecording() {
			ctx = observability.WithEventLabels(ctx, event)
			labeler, _ := otelhttp.LabelerFromContext(ctx)
			span.SetAttributes(labeler.Get()...)
		}
		span.End()
	}()

	statusCode, dispatchTime, err := env.send(ctx, event)
	if err != nil {
		logger.Errorw("failed to send event", zap.Error(err))
	}
	if dispatchTime > noDuration {
		labeler, _ := otelhttp.LabelerFromContext(ctx)
		env.dispatchDuration.Record(ctx, dispatchTime.Seconds(), metric.WithAttributes(labeler.Get()...))
	}

	writer.WriteHeader(statusCode)
}

func (env *envConfig) send(ctx context.Context, event *cloudevents.Event) (int, time.Duration, error) {
	headerCarrier := propagation.HeaderCarrier{}
	propagationContext.Inject(ctx, headerCarrier)
	tp := headerCarrier.Get("traceparent")
	ts := headerCarrier.Get("tracestate")
	channel := env.rmqHelper.GetChannel()
	if channel != nil {
		start := time.Now()
		dc, err := channel.PublishWithDeferredConfirm(
			env.ExchangeName,
			"",    // routing key
			false, // mandatory
			false, // immediate
			*rabbit.CloudEventToRabbitMQMessage(event, tp, ts))
		if err != nil {
			return http.StatusInternalServerError, noDuration, fmt.Errorf("failed to publish message: %w", err)
		}

		ack := dc.Wait()
		dispatchTime := time.Since(start)
		if !ack {
			return http.StatusInternalServerError, noDuration, errors.New("failed to publish message: nacked")
		}
		return http.StatusAccepted, dispatchTime, nil
	}
	return http.StatusInternalServerError, noDuration, errors.New("failed to publish message: RabbitMQ channel is nil")
}
