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

	wabbitamqp "github.com/NeowayLabs/wabbit/amqp"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
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

	connection *wabbitamqp.Conn
	channel    *wabbitamqp.Channel
}

func main() {
	ctx := signals.NewContext()
	retryNumber := 0
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var: ", err)
	}

	env.SetComponent(component)
	var err error
	logger := env.GetLogger()
	ctx = logging.WithLogger(ctx, logger)

	if err = env.SetupTracing(); err != nil {
		logger.Errorw("Failed setting up trace publishing", zap.Error(err))
	}

	if err = env.SetupMetrics(ctx); err != nil {
		logger.Errorw("Failed to create the metrics exporter", zap.Error(err))
	}

	retryChannel := make(chan bool)
	env.CreateRabbitMQResources(&retryNumber, retryChannel, logger)
	go func() {
		for {
			retry := <-retryChannel
			if !retry {
				return
			}
			env.CreateRabbitMQResources(&retryNumber, retryChannel, logger)
		}
	}()
	defer func() {
		retryChannel <- false
		close(retryChannel)
		rabbit.CleanupRabbitMQ(env.connection, env.channel, logger)
	}()

	connectionArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}
	kncloudevents.ConfigureConnectionArgs(&connectionArgs)

	receiver := kncloudevents.NewHTTPMessageReceiver(env.Port)

	if err := receiver.StartListen(ctx, &env); err != nil {
		logger.Fatalf("Failed to start listen, %v", err)
	}
}

func (env *envConfig) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	logger := env.GetLogger()
	// validate request method
	if request.Method != http.MethodPost {
		logger.Warn("Unexpected request method", zap.String("method", request.Method))
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// validate request URI
	if request.RequestURI != "/" {
		logger.Error("Unexpected incoming request uri")
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	ctx := request.Context()

	message := cehttp.NewMessageFromHttpRequest(request)
	defer message.Finish(nil)

	event, err := binding.ToEvent(ctx, message)
	if err != nil {
		logger.Warnw("Failed to extract event from request", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// run validation for the extracted event
	validationErr := event.Validate()
	if validationErr != nil {
		logger.Warnw("Failed to validate extracted event", zap.Error(validationErr))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	span := trace.FromContext(ctx)
	defer span.End()

	statusCode, err := env.send(event, span)
	if err != nil {
		logger.Errorw("Failed to send event", zap.Error(err))
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

func (env *envConfig) CreateRabbitMQResources(retryNumber *int, retryChannel chan<- bool, logger *zap.SugaredLogger) {
	conn, channel, err := rabbit.SetupRabbitMQ(env.BrokerURL, retryNumber, retryChannel, logger)
	if err != nil {
		return
	}
	env.connection = conn.(*wabbitamqp.Conn)
	env.channel = channel.(*wabbitamqp.Channel)
}
