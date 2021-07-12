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

package resources

import (
	"context"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/pkg/kmeta"

	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

const ChannelLabelKey = "messaging.knative.dev/channel"

func NewQueue(ctx context.Context, c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) *rabbitv1beta1.Queue {
	var queueName string
	if s != nil {
		queueName = naming.CreateSubscriberQueueName(c, s)
	} else {
		queueName = naming.CreateChannelDeadLetterQueueName(c)
	}
	q := &rabbitv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       c.Namespace,
			Name:            queueName,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(c)},
			Labels:          QueueLabels(c, s),
		},
		Spec: rabbitv1beta1.QueueSpec{
			// Why is the name in the Spec again? Is this different from the ObjectMeta.Name? If not,
			// maybe it should be removed?
			Name:       queueName,
			Durable:    true,
			AutoDelete: false,
			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			// TODO: This one has to exist in the same namespace as this exchange.
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: c.Spec.Config.Name,
			},
		},
	}
	if s != nil {
		// If the Subscriber has DeadLetterSink specified, we need to point to subscribers DLX instead of the channel
		if s.Delivery != nil && s.Delivery.DeadLetterSink != nil {
			q.Spec.Arguments = &runtime.RawExtension{
				Raw: []byte(`{"x-dead-letter-exchange":"` + naming.SubscriberDLXExchangeName(c, s) + `"}`),
			}

		} else {
			q.Spec.Arguments = &runtime.RawExtension{
				Raw: []byte(`{"x-dead-letter-exchange":"` + naming.ChannelExchangeName(c, true) + `"}`),
			}
		}
	}
	return q
}

func NewSubscriptionDLQ(ctx context.Context, c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) *rabbitv1beta1.Queue {
	queueName := naming.CreateSubscriberDeadLetterQueueName(c, s)
	return &rabbitv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       c.Namespace,
			Name:            queueName,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(c)},
			Labels:          QueueLabels(c, s),
		},
		Spec: rabbitv1beta1.QueueSpec{
			// Why is the name in the Spec again? Is this different from the ObjectMeta.Name? If not,
			// maybe it should be removed?
			Name:       queueName,
			Durable:    true,
			AutoDelete: false,
			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			// TODO: This one has to exist in the same namespace as this exchange.
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: c.Spec.Config.Name,
			},
		},
	}
}

// QueueLabels generates the labels present on the Queue linking the Broker / Trigger to the
// Queue.
func QueueLabels(c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) map[string]string {
	if s == nil {
		return map[string]string{
			"messaging.knative.dev/channel": c.Name,
		}
	} else {
		return map[string]string{
			"messaging.knative.dev/channel":      c.Name,
			"messaging.knative.dev/subscription": string(s.UID),
		}
	}
}
