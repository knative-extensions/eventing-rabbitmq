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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

type QueueArgs struct {
	Name      string
	Namespace string
	Broker    *eventingv1.Broker
	Owner     metav1.OwnerReference
	Labels    map[string]string
	DLXName   *string
}

func NewQueue(ctx context.Context, args *QueueArgs) *rabbitv1beta1.Queue {
	q := &rabbitv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Namespace,
			Name:            args.Name,
			OwnerReferences: []metav1.OwnerReference{args.Owner},
			Labels:          args.Labels,
		},
		Spec: rabbitv1beta1.QueueSpec{
			// Why is the name in the Spec again? Is this different from the ObjectMeta.Name? If not,
			// maybe it should be removed?
			Name:       args.Name,
			Durable:    true,
			AutoDelete: false,
			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name:      args.Broker.Spec.Config.Name,
				Namespace: args.Broker.Spec.Config.Namespace,
			},
		},
	}
	if args.DLXName != nil {
		q.Spec.Arguments = &runtime.RawExtension{
			Raw: []byte(fmt.Sprintf(`{"x-dead-letter-exchange":%q}`, *args.DLXName)),
		}
	}
	return q
}

// QueueLabels generates the labels present on the Queue linking the Broker / Trigger to the
// Queue.
func QueueLabels(b *eventingv1.Broker, t *eventingv1.Trigger) map[string]string {
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
