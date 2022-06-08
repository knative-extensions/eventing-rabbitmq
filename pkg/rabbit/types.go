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

package rabbit

import (
	"context"
	"net/url"

	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

type Result struct {
	Name  string
	Ready bool
}

type Service interface {
	RabbitMQURL(context.Context, *rabbitv1beta1.RabbitmqClusterReference) (*url.URL, error)
	ReconcileExchange(context.Context, *ExchangeArgs) (Result, error)
	ReconcileQueue(context.Context, *QueueArgs) (Result, error)
	ReconcileBinding(context.Context, *BindingArgs) (Result, error)
	ReconcileBrokerDLXPolicy(context.Context, *QueueArgs) (Result, error)
}
