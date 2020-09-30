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
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"

	"github.com/NeowayLabs/wabbit/amqp"
	"knative.dev/eventing-rabbitmq/pkg/dispatcher"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

type envConfig struct {
	QueueName        string `envconfig:"QUEUE_NAME" required:"true"`
	RabbitURL        string `envconfig:"RABBIT_URL" required:"true"`
	BrokerIngressURL string `envconfig:"BROKER_INGRESS_URL" required:"true"`
	SubscriberURL    string `envconfig:"SUBSCRIBER" required:"true"`
	// Should failed deliveries be requeued in the RabbitMQ?
	Requeue bool `envconfig:"REQUEUE" default: false required:"true"`

	Retry         int           `envconfig:"RETRY" required:"false"`
	BackoffPolicy string        `envconfig:"BACKOFF_POLICY" required:"false"`
	BackoffDelay  time.Duration `envconfig:"BACKOFF_DELAY" required:"false"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var: ", err)
	}

	var backoffPolicy eventingduckv1.BackoffPolicyType
	if env.BackoffPolicy == "" || env.BackoffPolicy == "exponential" {
		backoffPolicy = eventingduckv1.BackoffPolicyExponential
	} else if env.BackoffPolicy == "linear" {
		backoffPolicy = eventingduckv1.BackoffPolicyLinear
	} else {
		log.Fatal("Invalid BACKOFF_POLICY specified, must be exponential or linear")
	}

	ctx := signals.NewContext()

	conn, err := amqp.Dial(env.RabbitURL)
	if err != nil {
		logging.FromContext(ctx).Fatal("failed to connect to RabbitMQ: ", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		logging.FromContext(ctx).Fatal("failed to open a channel: ", err)
	}
	defer channel.Close()

	err = channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		logging.FromContext(ctx).Fatal("failed to create QoS: %s", err)
	}

	d := dispatcher.NewDispatcher(env.BrokerIngressURL, env.SubscriberURL, env.Requeue, env.Retry, env.BackoffDelay, backoffPolicy)
	d.ConsumeFromQueue(ctx, channel, env.QueueName)
}
