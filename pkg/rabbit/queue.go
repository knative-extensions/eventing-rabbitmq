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
	"fmt"
	"regexp"

	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

type QueueArgs struct {
	Name                     string
	Namespace                string
	QueueName                string
	RabbitmqClusterReference *rabbitv1beta1.RabbitmqClusterReference
	Owner                    metav1.OwnerReference
	Labels                   map[string]string
	DLXName                  *string
	Source                   *v1alpha1.RabbitmqSource
	BrokerUID                string
}

func NewQueue(args *QueueArgs) *rabbitv1beta1.Queue {
	// queue configurations for broker and trigger
	durable := true
	autoDelete := false
	queueName := args.Name
	vhost := "/"
	if args.QueueName != "" {
		queueName = args.QueueName
	}

	// queue configurations for source
	if args.Source != nil {
		durable = args.Source.Spec.QueueConfig.Durable
		autoDelete = args.Source.Spec.QueueConfig.AutoDelete
		queueName = args.Source.Spec.QueueConfig.Name
		vhost = args.Source.Spec.Vhost
	}

	return &rabbitv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Namespace,
			Name:            args.Name,
			OwnerReferences: []metav1.OwnerReference{args.Owner},
			Labels:          args.Labels,
		},
		Spec: rabbitv1beta1.QueueSpec{
			Name:                     queueName,
			Vhost:                    vhost,
			Durable:                  durable,
			AutoDelete:               autoDelete,
			RabbitmqClusterReference: *args.RabbitmqClusterReference,
		},
	}
}

func NewPolicy(args *QueueArgs) *rabbitv1beta1.Policy {
	return &rabbitv1beta1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            args.Name,
			Namespace:       args.Namespace,
			OwnerReferences: []metav1.OwnerReference{args.Owner},
			Labels:          args.Labels,
		},
		Spec: v1beta1.PolicySpec{
			Name:                     args.Name,
			Priority:                 1,
			Pattern:                  fmt.Sprintf("^%s$", regexp.QuoteMeta(args.Name)),
			ApplyTo:                  "queues",
			Definition:               &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"dead-letter-exchange": %q}`, *args.DLXName))},
			RabbitmqClusterReference: *args.RabbitmqClusterReference,
		},
	}
}

// NewBrokerDLXPolicy configures the broker dead letter exchange for trigger queues that does not have dlx defined
func NewBrokerDLXPolicy(args *QueueArgs) *rabbitv1beta1.Policy {
	return &rabbitv1beta1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            args.Name,
			Namespace:       args.Namespace,
			OwnerReferences: []metav1.OwnerReference{args.Owner},
			Labels:          args.Labels,
		},
		Spec: v1beta1.PolicySpec{
			Name:                     args.Name,
			Priority:                 0, // lower priority then policies created for trigger queues to allow overwrite
			Pattern:                  fmt.Sprintf("%s$", regexp.QuoteMeta(args.BrokerUID)),
			ApplyTo:                  "queues",
			Definition:               &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"dead-letter-exchange": %q}`, *args.DLXName))},
			RabbitmqClusterReference: *args.RabbitmqClusterReference,
		},
	}
}
