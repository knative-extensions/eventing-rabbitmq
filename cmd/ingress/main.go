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
	"errors"
	"fmt"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"knative.dev/eventing-rabbitmq/pkg/broker/ingress"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
)

const (
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 1000
	component                        = "rabbitmq-ingress"

	// noDuration signals that the dispatch step hasn't started
	noDuration = -1
)

type envConfig struct {
	utils.EnvConfig

	Port         int    `envconfig:"PORT" default:"8080"`
	BrokerURL    string `envconfig:"BROKER_URL" required:"true"`
	ExchangeName string `envconfig:"EXCHANGE_NAME" required:"true"`

	rmqHelper             rabbit.RabbitMQHelperInterface
	retryConnectionsSetup chan bool

	ContainerName   string `envconfig:"CONTAINER_NAME" default:"ingress"`
	PodName         string `envconfig:"POD_NAME" default:"rabbitmq-broker-ingress"`
	BrokerName      string `envconfig:"BROKER_NAME"`
	BrokerNamespace string `envconfig:"BROKER_NAMESPACE"`

	reporter ingress.StatsReporter
}

func main() {
	ctx := signals.NewContext()
	var err error

	// Report stats on Go memory usage every 30 seconds.
	metrics.MemStatsOrDie(ctx)

	var env envConfig
	if err = envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var: ", err)
	}

	env.SetComponent(component)
	logger := env.GetLogger()
	ctx = logging.WithLogger(ctx, logger)

	if err = env.SetupTracing(); err != nil {
		logger.Errorw("failed setting up trace publishing", zap.Error(err))
	}

	if err = env.SetupMetrics(ctx); err != nil {
		logger.Errorw("failed to create the metrics exporter", zap.Error(err))
	}

	env.retryConnectionsSetup = make(chan bool)
	env.rmqHelper = rabbit.NewRabbitMQHelper(1, logger, rabbit.DialWrapper)
	// Wait for RabbitMQ retry messages
	defer env.rmqHelper.CloseRabbitMQConnections()
	env.rmqHelper.SetupRabbitMQConnectionAndChannel(env.BrokerURL, rabbit.ChannelConfirm)
	go func() {
		for <-env.retryConnectionsSetup {
			env.rmqHelper.CloseRabbitMQConnections()
			env.rmqHelper.SetupRabbitMQConnectionAndChannel(env.BrokerURL, rabbit.ChannelConfirm)
		}
		logger.Info("stopped watching for rabbitmq connections")
	}()

	env.reporter = ingress.NewStatsReporter(env.ContainerName, kmeta.ChildName(env.PodName, uuid.New().String()))
	connectionArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}
	kncloudevents.ConfigureConnectionArgs(&connectionArgs)
	receiver := kncloudevents.NewHTTPMessageReceiver(env.Port)
	if err = receiver.StartListen(ctx, &env); err != nil {
		logger.Fatalf("failed to start listen, %v", err)
	}
	env.retryConnectionsSetup <- false
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

	span := trace.FromContext(ctx)
	defer span.End()

	reporterArgs := &ingress.ReportArgs{
		Namespace:  env.BrokerNamespace,
		BrokerName: env.BrokerName,
		EventType:  event.Type(),
	}

	statusCode, dispatchTime, err := env.send(event, span)
	if err != nil {
		logger.Errorw("failed to send event", zap.Error(err))
	}
	if dispatchTime > noDuration {
		_ = env.reporter.ReportEventDispatchTime(reporterArgs, statusCode, dispatchTime)
	}
	_ = env.reporter.ReportEventCount(reporterArgs, statusCode)

	writer.WriteHeader(statusCode)
}

func (env *envConfig) send(event *cloudevents.Event, span *trace.Span) (int, time.Duration, error) {
	tp, ts := (&tracecontext.HTTPFormat{}).SpanContextToHeaders(span.SpanContext())
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
			go func() { env.retryConnectionsSetup <- true }()
			return http.StatusInternalServerError, noDuration, fmt.Errorf("failed to publish message: %w", err)
		}

		ack := dc.Wait()
		dispatchTime := time.Since(start)
		if !ack {
			return http.StatusInternalServerError, noDuration, errors.New("failed to publish message: nacked")
		}
		return http.StatusAccepted, dispatchTime, nil
	}
	go func() { env.retryConnectionsSetup <- true }()
	return http.StatusInternalServerError, noDuration, errors.New("failed to publish message: RabbitMQ channel is nil")
}
