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
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

const (
	BindingKey           = "x-knative-trigger"
	DLQBindingKey        = "x-knative-dlq"
	TriggerDLQBindingKey = "x-knative-trigger-dlq"
)

type BindingArgs struct {
	Name                     string
	Namespace                string
	Owner                    metav1.OwnerReference
	RabbitmqClusterReference *rabbitv1beta1.RabbitmqClusterReference
	Vhost                    string
	Source                   string
	Destination              string
	Labels                   map[string]string
	Filters                  map[string]string
	ClusterName              string
}

func NewBinding(args *BindingArgs) (*rabbitv1beta1.Binding, error) {
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
			Vhost:           args.Vhost,
			Source:          args.Source,
			Destination:     args.Destination,
			DestinationType: "queue",
			RoutingKey:      "",
			Arguments: &runtime.RawExtension{
				Raw: argumentsJson,
			},
			RabbitmqClusterReference: *args.RabbitmqClusterReference,
		},
	}
	return binding, nil
}
