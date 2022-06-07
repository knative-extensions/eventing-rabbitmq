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

package rabbit

import (
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/kmeta"

	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

// ExchangeArgs are the arguments to create a RabbitMQ Exchange.
type ExchangeArgs struct {
	Name                     string
	Namespace                string
	RabbitmqClusterReference *rabbitv1beta1.RabbitmqClusterReference
	RabbitMQURL              *url.URL
	Broker                   *eventingv1.Broker
	Trigger                  *eventingv1.Trigger
	Source                   *v1alpha1.RabbitmqSource
}

// NewExchange returns an `exchange.rabbitmq.com` object
// used by trigger, broker, and source reconcilers
// when used by trigger and broker, exchange properties such as `durable`, autoDelete`, and `type` are hardcoded
func NewExchange(args *ExchangeArgs) *rabbitv1beta1.Exchange {
	// exchange configurations for triggers and broker
	vhost := "/"

	var exchangeName string
	var ownerReference metav1.OwnerReference
	if args.Trigger != nil {
		ownerReference = *kmeta.NewControllerRef(args.Trigger)
		exchangeName = args.Name
	} else if args.Broker != nil {
		ownerReference = *kmeta.NewControllerRef(args.Broker)
		exchangeName = args.Name
	} else if args.Source != nil {
		ownerReference = *kmeta.NewControllerRef(args.Source)
		exchangeName = args.Source.Spec.ExchangeConfig.Name
	}

	return &rabbitv1beta1.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Namespace,
			Name:            args.Name,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
			Labels:          Labels(args.Broker, args.Trigger, args.Source),
		},
		Spec: rabbitv1beta1.ExchangeSpec{
			Name:                     exchangeName,
			Vhost:                    vhost,
			Type:                     "headers",
			Durable:                  true,
			AutoDelete:               false,
			RabbitmqClusterReference: *args.RabbitmqClusterReference,
		},
	}
}
