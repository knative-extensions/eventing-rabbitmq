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
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/kelseyhightower/envconfig"

	"knative.dev/eventing-rabbitmq/pkg/dispatcher"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

type envConfig struct {
	utils.EnvConfig

	QueueName         string `envconfig:"QUEUE_NAME" required:"true"`
	RabbitURL         string `envconfig:"RABBIT_URL" required:"true"`
	RabbitMQVhost     string `envconfig:"RABBITMQ_VHOST" required:"false"`
	BrokerIngressURL  string `envconfig:"BROKER_INGRESS_URL" required:"true"`
	SubscriberURL     string `envconfig:"SUBSCRIBER" required:"true"`
	SubscriberCACerts string `envconfig:"SUBSCRIBER_CACERTS" required:"false"`
	DLX               bool   `envconfig:"DLX" required:"false"`
	DLXName           string `envconfig:"DLX_NAME" required:"false"`

	// Number of concurrent messages in flight
	Parallelism   int           `envconfig:"PARALLELISM" default:"1" required:"false"`
	Retry         int           `envconfig:"RETRY" required:"false"`
	BackoffPolicy string        `envconfig:"BACKOFF_POLICY" required:"false"`
	BackoffDelay  time.Duration `envconfig:"BACKOFF_DELAY" default:"50ms" required:"false"`
	Timeout       time.Duration `envconfig:"TIMEOUT" default:"0s" required:"false"`

	ContainerName string `envconfig:"CONTAINER_NAME"`
	PodName       string `envconfig:"POD_NAME"`
	Namespace     string `envconfig:"NAMESPACE"`
}

const (
	scopeName = "knative.dev/eventing-rabbitmq/cmd/dispatcher"
)

var (
	latencyBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
)

func main() {
	ctx := signals.NewContext()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var: ", err)
	}

	env.SetComponent(dispatcher.ComponentName)
	logger := env.GetLogger()
	ctx = logging.WithLogger(ctx, logger)
	if err := env.SetupObservability(ctx); err != nil {
		logger.Errorw("Failed setting up observability", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		env.ShutdownObservability(ctx)
	}()

	backoffPolicy := utils.SetBackoffPolicy(ctx, env.BackoffPolicy)
	if backoffPolicy == "" {
		logging.FromContext(ctx).Fatalf("Invalid BACKOFF_POLICY specified: must be %q or %q", eventingduckv1.BackoffPolicyExponential, eventingduckv1.BackoffPolicyLinear)
	}
	backoffDelay := env.BackoffDelay
	logging.FromContext(ctx).Infow("Setting BackoffDelay", zap.Any("backoffDelay", backoffDelay))

	d := &dispatcher.Dispatcher{
		BrokerIngressURL:  env.BrokerIngressURL,
		SubscriberURL:     env.SubscriberURL,
		SubscriberCACerts: env.SubscriberCACerts,
		MaxRetries:        env.Retry,
		BackoffDelay:      backoffDelay,
		Timeout:           env.Timeout,
		BackoffPolicy:     backoffPolicy,
		WorkerCount:       env.Parallelism,
		DLX:               env.DLX,
		DLXName:           env.DLXName,
		Tracer:            otel.GetTracerProvider().Tracer(scopeName),
	}

	meter := otel.GetMeterProvider().Meter(scopeName)

	var err error
	d.DispatchDuration, err = meter.Float64Histogram(
		"kn.eventing.dispatch.duration",
		metric.WithDescription("The time to dispatch the event"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBounds...),
	)
	if err != nil {
		logger.Fatalw("failed to set up dispatch metric", zap.Error(err))
	}

	rmqHelper := rabbit.NewRabbitMQConnectionHandler(5, 1000, logger)
	rmqHelper.Setup(ctx, rabbit.VHostHandler(
		env.RabbitURL,
		env.RabbitMQVhost),
		func(c rabbit.RabbitMQConnectionInterface, ch rabbit.RabbitMQChannelInterface) error {
			if err := rabbit.ChannelQoS(c, ch); err != nil {
				return err
			}

			if err := rabbit.ChannelConfirm(c, ch); err != nil {
				return err
			}
			return nil
		},
		rabbit.DialWrapper)
	for {
		if err = d.ConsumeFromQueue(ctx, rmqHelper.GetConnection(), rmqHelper.GetChannel(), env.QueueName); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logger.Error(err)
			time.Sleep(time.Second)
		}
	}
}
