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

	"github.com/kelseyhightower/envconfig"
	amqp "github.com/rabbitmq/amqp091-go"

	"knative.dev/eventing-rabbitmq/pkg/dispatcher"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
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

	connection *amqp.Connection
	channel    *amqp.Channel
}

func main() {
	ctx := signals.NewContext()
	var err error
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var: ", err)
	}

	env.SetComponent(dispatcher.ComponentName)
	logger := env.GetLogger()
	ctx = logging.WithLogger(ctx, logger)
	if err = env.SetupTracing(); err != nil {
		logger.Errorw("Failed setting up trace publishing", zap.Error(err))
	}
	if err = env.SetupMetrics(ctx); err != nil {
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

	rmqHelper := rabbit.NewRabbitMQHelper(1)
	retryChannel := make(chan bool)
	// Wait for RabbitMQ retry messages
	go rmqHelper.RetryHandler(env.CreateRabbitMQConnections, retryChannel, logger)
	env.CreateRabbitMQConnections(rmqHelper, retryChannel, logger)
	defer rmqHelper.CleanupRabbitMQ(env.connection, env.channel, retryChannel, logger)

	d := &dispatcher.Dispatcher{
		BrokerIngressURL: env.BrokerIngressURL,
		SubscriberURL:    env.SubscriberURL,
		MaxRetries:       env.Retry,
		BackoffDelay:     backoffDelay,
		BackoffPolicy:    backoffPolicy,
		WorkerCount:      env.Parallelism,
	}

	for {
		if err = d.ConsumeFromQueue(ctx, env.channel, env.QueueName); err != nil {
			// ignore ctx cancelled and channel closed errors
			if errors.Is(err, amqp.ErrClosed) {
				continue
			} else if errors.Is(err, context.Canceled) {
				return
			}

			logging.FromContext(ctx).Fatal("Failed to consume from queue: ", err)
			break
		}
	}
}

func (env *envConfig) CreateRabbitMQConnections(
	rmqHelper *rabbit.RabbitMQHelper,
	retryChannel chan<- bool,
	logger *zap.SugaredLogger) {
	conn, channel, err := rmqHelper.SetupRabbitMQ(env.RabbitURL, retryChannel, logger)
	if err == nil {
		err = env.channel.Qos(
			env.Parallelism, // prefetch count
			0,               // prefetch size
			false,           // global
		)
	}
	if err != nil {
		rmqHelper.CloseRabbitMQConnections(conn, channel, logger)
		go rmqHelper.SignalRetry(retryChannel, true)
		if err != nil {
			env.connection, env.channel = nil, nil
			logger.Errorf("error recreating RabbitMQ connections: %s, waiting for a retry", err)
		}
		env.connection, env.channel = nil, nil
	}
	env.connection, env.channel = conn, channel
}
