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

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	"knative.dev/eventing-rabbitmq/pkg/dispatcher"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/signals"
)

type envConfig struct {
	QueueName        string `envconfig:"QUEUE_NAME" required:"true"`
	BrokerURL        string `envconfig:"BROKER_URL" required:"true"`
	BrokerIngressURL string `envconfig:"BROKER_INGRESS_URL" required:"true"`
	SubscriberURL    string `envconfig:"SUBSCRIBER" required:"true"`

	Retry int32 `envconfig:"RETRY" default: 3, required:"false"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	delivery := &eventingduckv1.DeliverySpec{
		Retry: &env.Retry,
	}

	sctx := signals.NewContext()

	d := dispatcher.NewDispatcher(env.QueueName, env.BrokerURL, env.BrokerIngressURL, env.SubscriberURL, delivery)
	d.Start(sctx)

}
