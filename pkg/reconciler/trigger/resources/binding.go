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
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	BindingKey           = "x-knative-trigger"
	DLQBindingKey        = "x-knative-dlq"
	TriggerDLQBindingKey = "x-knative-trigger-dlq"
)

type BindingArgs struct {
	Name                     string
	Namespace                string
	Broker                   *eventingv1.Broker
	RabbitMQClusterName      string
	RabbitMQClusterNamespace string
	Source                   string
	Destination              string
	Owner                    metav1.OwnerReference
	Labels                   map[string]string
	Filters                  map[string]string
	ClusterName              string
}

func NewBinding(ctx context.Context, args *BindingArgs) (*rabbitv1beta1.Binding, error) {
	if args.Filters == nil {
		args.Filters = map[string]string{}
	}
	args.Filters["x-match"] = "all"

	argumentsJson, err := json.Marshal(args.Filters)
	if err != nil {
		return nil, fmt.Errorf("failed to encode binding arguments %+v : %s", argumentsJson, err)
	}

	binding := &rabbitv1beta1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Namespace,
			Name:            args.Name,
			OwnerReferences: []metav1.OwnerReference{args.Owner},
			Labels:          args.Labels,
		},
		Spec: rabbitv1beta1.BindingSpec{
			Vhost:           "/",
			Source:          args.Source,
			Destination:     args.Destination,
			DestinationType: "queue",
			RoutingKey:      "",
			Arguments: &runtime.RawExtension{
				Raw: argumentsJson,
			},
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name:      args.RabbitMQClusterName,
				Namespace: args.RabbitMQClusterNamespace,
			},
		},
	}
	return binding, nil
}

// BindingLabels generates the labels present on the Queue linking the Broker / Trigger to the
// Binding.
func BindingLabels(b *eventingv1.Broker, t *eventingv1.Trigger) map[string]string {
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
