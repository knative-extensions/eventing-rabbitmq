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
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestBrokerImmutableFields(t *testing.T) {
	original := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"eventing.knative.dev/broker.class": "original"},
		},
		Spec: eventingv1.BrokerSpec{
			Config: &duckv1.KReference{
				Namespace:  "namespace",
				Name:       "name",
				Kind:       "Secret",
				APIVersion: "v1",
			},
		},
	}
	current := &RabbitBroker{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
		},
		Spec: eventingv1.BrokerSpec{
			Config: &duckv1.KReference{
				Namespace:  "namespace",
				Name:       "name",
				Kind:       "Secret",
				APIVersion: "v1",
			},
		},
	}
	currentRealBroker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
		},
		Spec: eventingv1.BrokerSpec{
			Config: &duckv1.KReference{
				Namespace:  "namespace",
				Name:       "name",
				Kind:       "Secret",
				APIVersion: "v1",
			},
		},
	}
	originalValid := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
		},
		Spec: eventingv1.BrokerSpec{
			Config: &duckv1.KReference{
				Namespace:  "namespace",
				Name:       "name",
				Kind:       "RabbitmqCluster",
				APIVersion: "rabbitmq.com/v1beta1",
			},
		},
	}

	tests := map[string]struct {
		og      *eventingv1.Broker
		wantErr *apis.FieldError
	}{
		"nil original": {
			wantErr: nil,
		},
		"no BrokerClassAnnotation mutation": {
			og:      currentRealBroker,
			wantErr: nil,
		},
		"BrokerClassAnnotation mutated": {
			og: original,
			wantErr: &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"annotations"},
				Details: `{string}:
	-: "original"
	+: "RabbitMQBroker"
`,
			},
		},
		"Config mutated": {
			og: originalValid,
			wantErr: &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"spec"},
				Details: `{v1.BrokerSpec}.Config.Kind:
	-: "RabbitmqCluster"
	+: "Secret"
{v1.BrokerSpec}.Config.APIVersion:
	-: "rabbitmq.com/v1beta1"
	+: "v1"
`,
			},
		},
	}

	for n, test := range tests {
		t.Run(n, func(t *testing.T) {
			ctx := context.Background()
			ctx = apis.WithinUpdate(ctx, test.og)
			got := current.Validate(ctx)
			if diff := cmp.Diff(test.wantErr.Error(), got.Error()); diff != "" {
				t.Error("Broker.CheckImmutableFields (-want, +got) =", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		b    RabbitBroker
		want *apis.FieldError
	}{{
		name: "missing annotation, so not our broker",
		b:    RabbitBroker{},
	}, {
		name: "empty annotation, again not our broker",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": ""},
			},
		},
	}, {
		name: "empty, missing config",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
			},
		},
		want: apis.ErrMissingField("spec.config"),
	}, {
		name: "valid config, just some other BrokerClass",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "notourbrokerclass"},
			},
			Spec: eventingv1.BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
	}, {
		name: "invalid config",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
			},
			Spec: eventingv1.BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
		want: apis.ErrGeneric("Configuration not supported, only [kind: RabbitmqCluster, apiVersion: rabbitmq.com/v1beta1] or [kind: RabbitmqBrokerConfig, apiVersion: eventing.knative.dev/v1alpha1]").ViaField("spec").ViaField("config"),
	}, {
		name: "invalid config, no namespace",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
			},
			Spec: eventingv1.BrokerSpec{
				Config: &duckv1.KReference{
					Name:       "name",
					Kind:       "RabbitmqCluster",
					APIVersion: "rabbitmq.com/v1beta1",
				},
			},
		},
		want: apis.ErrMissingField("spec.config.namespace"),
	}, {
		name: "invalid config, missing name",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
			},
			Spec: eventingv1.BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Kind:       "RabbitmqCluster",
					APIVersion: "rabbitmq.com/v1beta1",
				},
			},
		},
		want: apis.ErrMissingField("spec.config.name"),
	}, {
		name: "invalid config, missing apiVersion",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
			},
			Spec: eventingv1.BrokerSpec{
				Config: &duckv1.KReference{
					Namespace: "namespace",
					Name:      "name",
					Kind:      "RabbitmqCluster",
				},
			},
		},
		want: apis.ErrMissingField("spec.config.apiVersion"),
	}, {
		name: "invalid config, missing kind",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
			},
			Spec: eventingv1.BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name",
					APIVersion: "apiversion",
				},
			},
		},
		want: apis.ErrMissingField("spec.config.kind"),
	}, {
		name: "valid config, rabbitmqcluster",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
			},
			Spec: eventingv1.BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "othernamespace",
					Name:       "name",
					Kind:       "RabbitmqCluster",
					APIVersion: "rabbitmq.com/v1beta1",
				},
			},
		},
	}, {
		name: "valid config, rabbitmqBrokerConfig",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "RabbitMQBroker"},
			},
			Spec: eventingv1.BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "othernamespace",
					Name:       "name",
					Kind:       "RabbitmqBrokerConfig",
					APIVersion: "eventing.knative.dev/v1alpha1",
				},
			},
		},
	}, {
		name: "invalid resource annotations",
		b: RabbitBroker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"eventing.knative.dev/broker.class": "RabbitMQBroker",
					utils.CPURequestAnnotation:          "invalid",
				},
			},
		},
		want: &apis.FieldError{
			Message: "Failed to parse quantity from rabbitmq.eventing.knative.dev/cpu-request",
			Paths:   []string{"metadata", "annotations", "rabbitmq.eventing.knative.dev/cpu-request"},
			Details: "quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'",
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.b.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("Broker.Validate (-want, +got) =", diff)
			}
		})
	}
}
