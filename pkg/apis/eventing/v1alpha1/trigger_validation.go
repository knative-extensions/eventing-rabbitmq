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

package v1alpha1

import (
	"context"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/webhook/resourcesemantics"
)

const (
	BrokerClass           = "RabbitMQBroker"
	parallelismAnnotation = "rabbitmq.eventing.knative.dev/parallelism"
)

// +k8s:controller-gen=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RabbitTrigger struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec eventingv1.TriggerSpec `json:"spec,omitempty"`

	// +optional
	Status eventingv1.TriggerStatus `json:"status,omitempty"`
}

var _ resourcesemantics.GenericCRD = (*RabbitTrigger)(nil)

func (t *RabbitTrigger) Validate(ctx context.Context) *apis.FieldError {
	c := client.Get(ctx)
	broker, _ := c.EventingV1().Brokers(t.Namespace).Get(ctx, t.Spec.Broker, metav1.GetOptions{})

	if broker != nil {
		bc, ok := broker.GetAnnotations()[eventingv1.BrokerClassAnnotationKey]
		if !ok || bc != BrokerClass {
			// Not my broker
			return nil
		}
	}

	// if parallelism is set, validate it
	parallelism, ok := t.GetAnnotations()[parallelismAnnotation]
	if ok {
		parallelismInt, err := strconv.Atoi(parallelism)
		if err != nil {
			return &apis.FieldError{
				Message: "Failed to parse valid int from parallelismAnnotation",
				Paths:   []string{"metadata", "annotations", parallelismAnnotation},
				Details: err.Error(),
			}
		}

		if parallelismInt < 1 || parallelismInt > 1000 {
			return apis.ErrOutOfBoundsValue(parallelismInt, 1, 1000, parallelismAnnotation)
		}
	}

	if apiErr := utils.ValidateResourceRequestsAndLimits(t.ObjectMeta); apiErr != nil {
		return apiErr
	}

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*RabbitTrigger)
		if diff, err := kmp.ShortDiff(original.Spec.Filter, t.Spec.Filter); err != nil {
			return &apis.FieldError{
				Message: "Failed to diff Trigger",
				Paths:   []string{"spec", "filter"},
				Details: err.Error(),
			}
		} else if diff != "" {
			return &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"spec", "filter"},
				Details: diff,
			}
		}
	}

	return nil
}

func (t *RabbitTrigger) SetDefaults(ctx context.Context) {}
