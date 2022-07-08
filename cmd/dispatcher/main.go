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

	"github.com/NeowayLabs/wabbit"
	"github.com/NeowayLabs/wabbit/amqp"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	amqperr "github.com/rabbitmq/amqp091-go"
	"knative.dev/pkg/kmeta"

	dispatcherstats "knative.dev/eventing-rabbitmq/pkg/broker/dispatcher"
	"knative.dev/eventing-rabbitmq/pkg/dispatcher"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
)

type envConfig struct {
	utils.EnvConfig

	QueueName        string `envconfig:"QUEUE_NAME" required:"true"`
	RabbitURL        string `envconfig:"RABBIT_URL" required:"true"`
	BrokerIngressURL string `envconfig:"BROKER_INGRESS_URL" required:"true"`
	SubscriberURL    string `envconfig:"SUBSCRIBER" required:"true"`

	// Number of concurrent messages in flight
	Parallelism   int           `envconfig:"PARALLELISM" default:"1" required:"false"`
	Retry         int           `envconfig:"RETRY" required:"false"`
	BackoffPolicy string        `envconfig:"BACKOFF_POLICY" required:"false"`
	BackoffDelay  time.Duration `envconfig:"BACKOFF_DELAY" default:"50ms" required:"false"`
	Timeout       time.Duration `envconfig:"TIMEOUT" default:"0s" required:"false"`

	ContainerName string `envconfig:"CONTAINER_NAME"`
	PodName       string `envconfig:"POD_NAME"`
	Namespace     string `envconfig:"NAMESPACE"`

	connection *amqp.Conn
	channel    wabbit.Channel
}

func main() {
	ctx := signals.NewContext()

	// Report stats on Go memory usage every 30 seconds.
	metrics.MemStatsOrDie(ctx)

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

	backoffPolicy := utils.SetBackoffPolicy(ctx, env.BackoffPolicy)
	if backoffPolicy == "" {
		logging.FromContext(ctx).Fatalf("Invalid BACKOFF_POLICY specified: must be %q or %q", eventingduckv1.BackoffPolicyExponential, eventingduckv1.BackoffPolicyLinear)
	}
	backoffDelay := env.BackoffDelay
	logging.FromContext(ctx).Infow("Setting BackoffDelay", zap.Any("backoffDelay", backoffDelay))

	env.setupRabbitMQ(ctx)
	defer func() {
		err := env.connection.Close()
		if err != nil && !errors.Is(err, amqperr.ErrClosed) {
			logging.FromContext(ctx).Warn("Failed to close connection: ", err)
		}
	}()
	defer func() {
		err := env.channel.Close()
		if err != nil && !errors.Is(err, amqperr.ErrClosed) {
			logging.FromContext(ctx).Warn("Failed to close channel: ", err)
		}
	}()

	reporter := dispatcherstats.NewStatsReporter(env.ContainerName, kmeta.ChildName(env.PodName, uuid.New().String()), env.Namespace)
	d := &dispatcher.Dispatcher{
		BrokerIngressURL: env.BrokerIngressURL,
		SubscriberURL:    env.SubscriberURL,
		MaxRetries:       env.Retry,
		BackoffDelay:     backoffDelay,
		Timeout:          env.Timeout,
		BackoffPolicy:    backoffPolicy,
		WorkerCount:      env.Parallelism,
		Reporter:         reporter,
	}

	for {
		if err := d.ConsumeFromQueue(ctx, env.channel, env.QueueName); err != nil {
			// ignore ctx cancelled and channel closed errors
			if errors.Is(err, amqperr.ErrClosed) {
				env.setupRabbitMQ(ctx)
				continue
			}

			if errors.Is(err, context.Canceled) {
				return
			}

			logging.FromContext(ctx).Fatal("Failed to consume from queue: ", err)
			break
		}
	}
}

func (env *envConfig) setupRabbitMQ(ctx context.Context) {
	var err error

	if env.connection == nil || env.connection.IsClosed() {
		env.connection, err = amqp.Dial(env.RabbitURL)
		if err != nil {
			logging.FromContext(ctx).Fatal("Failed to connect to RabbitMQ: ", err)
		}
	}

	env.channel, err = env.connection.Channel()
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to open a channel: ", err)
	}

	err = env.channel.Qos(
		100,   // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to create QoS: ", err)
	}
}
