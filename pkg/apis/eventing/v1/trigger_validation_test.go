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

package v1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "knative.dev/eventing-rabbitmq/pkg/apis/eventing/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/client/clientset/versioned/fake"
	"knative.dev/pkg/apis"
)

func TestTriggerValidate(t *testing.T) {
	tests := []struct {
		name string
		trigger,
		original *v1.RabbitTrigger
		err *apis.FieldError
	}{
		{
			name:    "broker not found gets ignored",
			trigger: trigger(withBroker("foo")),
		},
		{
			name: "different broker class gets ignored",
			trigger: trigger(
				withBroker("foo"),
				withClientObjects(&eventingv1.Broker{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "foo",
						Annotations: map[string]string{eventingv1.BrokerClassAnnotationKey: "some-other-broker"},
					},
				})),
		},
		{
			name:     "filters are immutable",
			trigger:  trigger(withBroker("foo"), brokerExistsAndIsValid),
			original: trigger(withBroker("foo"), brokerExistsAndIsValid, withFilters(filter("x", "y"))),
			err: &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"spec", "filter"},
				Details: "{*v1.TriggerFilter}:\n\t-: \"&{Attributes:map[x:y]}\"\n\t+: \"<nil>\"\n",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.original != nil {
				ctx = apis.WithinUpdate(ctx, &tc.original.Trigger)
			}
			err := tc.trigger.Validate(ctx)
			if diff := cmp.Diff(tc.err, err, cmpopts.IgnoreUnexported(apis.FieldError{})); diff != "" {
				t.Error("Trigger.Validate (-want, +got) =", diff)
			}
		})
	}
}

type triggerOpt func(*v1.RabbitTrigger)

func trigger(opts ...triggerOpt) *v1.RabbitTrigger {
	t := &v1.RabbitTrigger{
		Trigger: eventingv1.Trigger{
			Spec: eventingv1.TriggerSpec{},
		},
		Client: fake.NewSimpleClientset(),
	}
	for _, o := range opts {
		o(t)
	}
	return t
}
func withClientObjects(objects ...runtime.Object) triggerOpt {
	return func(t *v1.RabbitTrigger) {
		t.Client = fake.NewSimpleClientset(objects...)
	}
}

func withBroker(name string) triggerOpt {
	return func(t *v1.RabbitTrigger) {
		t.Spec.Broker = name
	}
}

func brokerExistsAndIsValid(t *v1.RabbitTrigger) {
	withClientObjects(&eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:        t.Spec.Broker,
			Annotations: map[string]string{eventingv1.BrokerClassAnnotationKey: v1.BrokerClass},
		},
	})(t)
}

func filter(k, v string) []string {
	return []string{k, v}
}

func withFilters(filters ...[]string) triggerOpt {
	return func(t *v1.RabbitTrigger) {
		if t.Spec.Filter == nil {
			t.Spec.Filter = &eventingv1.TriggerFilter{}
		}
		if t.Spec.Filter.Attributes == nil {
			t.Spec.Filter.Attributes = map[string]string{}
		}
		for _, filter := range filters {
			t.Spec.Filter.Attributes[filter[0]] = filter[1]
		}
	}
}
