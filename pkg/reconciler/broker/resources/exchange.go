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

package resources

import (
	"context"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

// ExchangeArgs are the arguments to create a RabbitMQ Exchange.
type ExchangeArgs struct {
	Name            string
	Namespace       string
	Broker          *eventingv1.Broker
	Trigger         *eventingv1.Trigger
	RabbitMQURL     *url.URL
	RabbitMQCluster string
}

func NewExchange(ctx context.Context, args *ExchangeArgs) *rabbitv1beta1.Exchange {
	var or metav1.OwnerReference

	if args.Trigger != nil {
		or = *kmeta.NewControllerRef(args.Trigger)
	} else {
		or = *kmeta.NewControllerRef(args.Broker)
	}
	return &rabbitv1beta1.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Namespace,
			Name:            args.Name,
			OwnerReferences: []metav1.OwnerReference{or},
			Labels:          ExchangeLabels(args.Broker, args.Trigger),
		},
		Spec: rabbitv1beta1.ExchangeSpec{
			// Why is the name in the Spec again? Is this different from the ObjectMeta.Name? If not,
			// maybe it should be removed?
			Name:       args.Name,
			Type:       "headers",
			Durable:    true,
			AutoDelete: false,
			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			// TODO: This one has to exist in the same namespace as this exchange.
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: args.Broker.Spec.Config.Name,
			},
		},
	}
}

// ExchangeLabels generates the labels present on the Exchange linking the Broker to the
// Exchange.
func ExchangeLabels(b *eventingv1.Broker, t *eventingv1.Trigger) map[string]string {
	if t != nil {
		return map[string]string{
			"eventing.knative.dev/broker":  b.Name,
			"eventing.knative.dev/trigger": t.Name,
		}
	} else {
		return map[string]string{
			"eventing.knative.dev/broker": b.Name,
		}
	}
}
