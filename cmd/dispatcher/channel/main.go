/*
Copyright 2021 The Knative Authors

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

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	"github.com/NeowayLabs/wabbit/amqp"
	amqperr "github.com/streadway/amqp"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"

	"knative.dev/eventing-rabbitmq/pkg/dispatcher"
)

type envConfig struct {
	QueueName string `envconfig:"QUEUE_NAME" required:"true"`
	RabbitURL string `envconfig:"RABBIT_URL" required:"true"`
	// For channel, this is where Reply is sent if configured
	ChannelReplyURL string `envconfig:"REPLY_URL" required:"false"`
	SubscriberURL   string `envconfig:"SUBSCRIBER" required:"true"`
	// Should failed deliveries be requeued in the RabbitMQ?
	Requeue bool `envconfig:"REQUEUE" default:"false" required:"true"`

	Retry         int           `envconfig:"RETRY" required:"false"`
	BackoffPolicy string        `envconfig:"BACKOFF_POLICY" required:"false"`
	BackoffDelay  time.Duration `envconfig:"BACKOFF_DELAY" required:"false"`
}

const (
	defaultBackoffDelay = 50 * time.Millisecond
	defaultPrefetch     = 1
	defaultPrefetchSize = 0
)

func main() {
	ctx := signals.NewContext()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var: ", err)
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
	if backoffDelay == time.Duration(0) {
		logging.FromContext(ctx).Infow("BackoffDelay is zero, enforcing default", zap.Any("default", defaultBackoffDelay))
		backoffDelay = defaultBackoffDelay
	}
	logging.FromContext(ctx).Infow("Setting BackoffDelay", zap.Any("backoffDelay", backoffDelay))

	conn, err := amqp.Dial(env.RabbitURL)
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to connect to RabbitMQ: ", err)
	}
	defer func() {
		err = conn.Close()
		if err != nil {
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
		defaultPrefetch,     // prefetch count
		defaultPrefetchSize, // prefetch size
		false,               // global
	)
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to create QoS: ", err)
	}

	d := dispatcher.NewDispatcher(env.ChannelReplyURL, env.SubscriberURL, env.Requeue, env.Retry, backoffDelay, backoffPolicy)
	if err := d.ConsumeFromQueue(ctx, channel, env.QueueName); err != nil {
		// ignore ctx cancelled and channel closed errors
		if errors.Is(err, context.Canceled) || errors.Is(err, amqperr.ErrClosed) {
			return
		}
		logging.FromContext(ctx).Fatal("Failed to consume from queue: ", err)
	}
}
