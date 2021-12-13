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

	"go.uber.org/zap"

	"github.com/NeowayLabs/wabbit/amqp"
	"github.com/kelseyhightower/envconfig"
	amqperr "github.com/rabbitmq/amqp091-go"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"

	"knative.dev/eventing-rabbitmq/pkg/dispatcher"
	"knative.dev/eventing-rabbitmq/pkg/utils"
)

type envConfig struct {
	utils.EnvConfig

	QueueName        string `envconfig:"QUEUE_NAME" required:"true"`
	RabbitURL        string `envconfig:"RABBIT_URL" required:"true"`
	BrokerIngressURL string `envconfig:"BROKER_INGRESS_URL" required:"true"`
	SubscriberURL    string `envconfig:"SUBSCRIBER" required:"true"`
	// Should failed deliveries be requeued in the RabbitMQ?
	Requeue bool `envconfig:"REQUEUE" default:"false" required:"true"`

	// Number of concurrent messages in flight
	PrefetchCount int           `envconfig:"PREFETCH_COUNT" default:"10" required:"false"`
	Retry         int           `envconfig:"RETRY" required:"false"`
	BackoffPolicy string        `envconfig:"BACKOFF_POLICY" required:"false"`
	BackoffDelay  time.Duration `envconfig:"BACKOFF_DELAY" default:"50ms" required:"false"`
}

func main() {
	ctx := signals.NewContext()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var: ", err)
	}

	env.SetComponent(dispatcher.ComponentName)
	logger := env.GetLogger()
	ctx = logging.WithLogger(ctx, logger)
	if err := env.SetupTracing(); err != nil {
		logger.Errorw("Failed setting up trace publishing", zap.Error(err))
	}
	if err := env.SetupMetrics(ctx); err != nil {
		logger.Errorw("Failed to create the metrics exporter", zap.Error(err))
	}

	var backoffPolicy eventingduckv1.BackoffPolicyType
	if env.BackoffPolicy == "" || env.BackoffPolicy == "exponential" {
		backoffPolicy = eventingduckv1.BackoffPolicyExponential
	} else if env.BackoffPolicy == "linear" {
		backoffPolicy = eventingduckv1.BackoffPolicyLinear
	} else {
		logging.FromContext(ctx).Fatalf("Invalid BACKOFF_POLICY specified: must be %q or %q", eventingduckv1.BackoffPolicyExponential, eventingduckv1.BackoffPolicyLinear)
	}

	backoffDelay := env.BackoffDelay
	logging.FromContext(ctx).Infow("Setting BackoffDelay", zap.Any("backoffDelay", backoffDelay))

	conn, err := amqp.Dial(env.RabbitURL)
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to connect to RabbitMQ: ", err)
	}
	defer func() {
		err = conn.Close()
		if err != nil && !errors.Is(err, amqperr.ErrClosed) {
			logging.FromContext(ctx).Warn("Failed to close connection: ", err)
		}
	}()

	channel, err := conn.Channel()
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to open a channel: ", err)
	}
	defer func() {
		err = channel.Close()
		if err != nil && !errors.Is(err, amqperr.ErrClosed) {
			logging.FromContext(ctx).Warn("Failed to close channel: ", err)
		}
	}()

	err = channel.Qos(
		env.PrefetchCount, // prefetch count
		0,                 // prefetch size
		false,             // global
	)
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to create QoS: ", err)
	}

	d := &dispatcher.Dispatcher{
		BrokerIngressURL: env.BrokerIngressURL,
		SubscriberURL:    env.SubscriberURL,
		Requeue:          env.Requeue,
		MaxRetries:       env.Retry,
		BackoffDelay:     backoffDelay,
		BackoffPolicy:    backoffPolicy,
		WorkerCount:      env.PrefetchCount,
	}
	if err := d.ConsumeFromQueue(ctx, channel, env.QueueName); err != nil {
		// ignore ctx cancelled and channel closed errors
		if errors.Is(err, context.Canceled) || errors.Is(err, amqperr.ErrClosed) {
			return
		}
		logging.FromContext(ctx).Fatal("Failed to consume from queue: ", err)
	}
}
