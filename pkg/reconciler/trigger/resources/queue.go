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
	"fmt"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	brokerresources "knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/pkg/kmeta"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const TriggerLabelKey = "eventing.knative.dev/trigger"

// QueueArgs are the arguments to create a Trigger's RabbitMQ Queue.
type QueueArgs struct {
	QueueName       string
	RabbitmqURL     string
	RabbitmqCluster string
	// If the queue is for Trigger, this holds the trigger so we can create a proper Owner Ref
	Trigger *eventingv1.Trigger
	// If non-empty, wire the queue into this DLX.
	DLX string
}

func NewQueue(ctx context.Context, b *eventingv1.Broker, t *eventingv1.Trigger) *rabbitv1beta1.Queue {
	var or metav1.OwnerReference
	var queueName string
	if t != nil {
		or = *kmeta.NewControllerRef(t)
		queueName = CreateTriggerQueueName(t)
	} else {
		or = *kmeta.NewControllerRef(b)
		queueName = CreateBrokerDeadLetterQueueName(b)
	}
	q := &rabbitv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       b.Namespace,
			Name:            queueName,
			OwnerReferences: []metav1.OwnerReference{or},
			Labels:          QueueLabels(b, t),
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
				Name: b.Spec.Config.Name,
			},
		},
	}
	if t != nil {
		q.Spec.Arguments = &runtime.RawExtension{
			Raw: []byte(`{"x-dead-letter-exchange":"` + brokerresources.ExchangeName(b, true) + `"}`),
		}
	}
	return q
}

// QueueLabels generates the labels present on the Queue linking the Broker / Trigger to the
// Queue.
func QueueLabels(b *eventingv1.Broker, t *eventingv1.Trigger) map[string]string {
	if t == nil {
		return map[string]string{
			eventing.BrokerLabelKey: b.Name,
		}
	} else {
		return map[string]string{
			eventing.BrokerLabelKey: b.Name,
			TriggerLabelKey:         t.Name,
		}
	}
}

func CreateBrokerDeadLetterQueueName(b *eventingv1.Broker) string {
	// TODO(vaikas): https://github.com/knative-sandbox/eventing-rabbitmq/issues/61
	// return fmt.Sprintf("%s/%s/DLQ", b.Namespace, b.Name)
	return fmt.Sprintf("%s.%s.dlq", b.Namespace, b.Name)
}

func CreateTriggerQueueName(t *eventingv1.Trigger) string {
	// TODO(vaikas): https://github.com/knative-sandbox/eventing-rabbitmq/issues/61
	// return fmt.Sprintf("%s/%s", t.Namespace, t.Name)
	return fmt.Sprintf("%s.%s", t.Namespace, t.Name)
}
