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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

const (
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 1000
	component                        = "rabbitmq-ingress"
)

type envConfig struct {
	utils.EnvConfig

	Port         int    `envconfig:"PORT" default:"8080"`
	BrokerURL    string `envconfig:"BROKER_URL" required:"true"`
	ExchangeName string `envconfig:"EXCHANGE_NAME" required:"true"`

	channel *amqp.Channel
}

func main() {
	ctx := signals.NewContext()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var: ", err)
	}

	env.SetComponent(component)
	logger := env.GetLogger()
	ctx = logging.WithLogger(ctx, logger)
	if err := env.SetupTracing(); err != nil {
		logger.Errorw("Failed setting up trace publishing", zap.Error(err))
	}
	if err := env.SetupMetrics(ctx); err != nil {
		logger.Errorw("Failed to create the metrics exporter", zap.Error(err))
	}

	conn, err := amqp.Dial(env.BrokerURL)
	if err != nil {
		logger.Fatalw("failed to connect to RabbitMQ", zap.Error(err))
	}
	defer conn.Close()

	env.channel, err = conn.Channel()
	if err != nil {
		logger.Fatalw("failed to open a channel", zap.Error(err))
	}

	// noWait is false
	if err := env.channel.Confirm(false); err != nil {
		logger.Fatalf("faild to switch connection channel to confirm mode: %s", err)
	}
	defer env.channel.Close()

	connectionArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}
	kncloudevents.ConfigureConnectionArgs(&connectionArgs)

	receiver := kncloudevents.NewHTTPMessageReceiver(env.Port)

	if err := receiver.StartListen(ctx, &env); err != nil {
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

	statusCode, err := env.send(event, span)
	if err != nil {
		logger.Errorw("failed to send event", zap.Error(err))
	}
	writer.WriteHeader(statusCode)
}

func (env *envConfig) send(event *cloudevents.Event, span *trace.Span) (int, error) {
	bytes, err := json.Marshal(event)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to marshal event, %w", err)
	}
	tp, ts := (&tracecontext.HTTPFormat{}).SpanContextToHeaders(span.SpanContext())

	headers := amqp.Table{
		"type":        event.Type(),
		"source":      event.Source(),
		"subject":     event.Subject(),
		"traceparent": tp,
		"tracestate":  ts,
	}
	for key, val := range event.Extensions() {
		headers[key] = val
	}

	dc, err := env.channel.PublishWithDeferredConfirm(
		env.ExchangeName,
		"",    // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:      headers,
			ContentType:  "application/json",
			Body:         bytes,
			DeliveryMode: amqp.Persistent,
		})
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to publish message: %w", err)
	}

	ack := dc.Wait()
	if !ack {
		return http.StatusInternalServerError, errors.New("failed to publish message: nacked")
	}
	return http.StatusAccepted, nil
}
