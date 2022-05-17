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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/kmp"
)

// +k8s:deepcopy-gen=false
type RabbitBroker struct {
	eventingv1.Broker
}

var (
	_ apis.Validatable = (*RabbitBroker)(nil)
)

func (b *RabbitBroker) Validate(ctx context.Context) *apis.FieldError {
	bc, ok := b.GetAnnotations()[eventingv1.BrokerClassAnnotationKey]
	if !ok || bc != "RabbitMQBroker" {
		// Not my broker
		return nil
	}

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*eventingv1.Broker)
		if original != nil {
			// If the original is not my type or missing, complain
			if origBc, ok := original.GetAnnotations()[eventingv1.BrokerClassAnnotationKey]; !ok || origBc != "RabbitMQBroker" {
				// Spec is immutable, so fail it.
				return &apis.FieldError{
					Message: "Immutable fields changed (-old +new)",
					Paths:   []string{"annotations"},
					Details: fmt.Sprintf("{string}:\n\t-: %q\n\t+: %q\n", origBc, bc),
				}

			}
			if diff, err := kmp.ShortDiff(original.Spec, b.Spec); err != nil {
				return &apis.FieldError{
					Message: "Failed to diff Broker",
					Paths:   []string{"spec"},
					Details: err.Error(),
				}
			} else if diff != "" {
				// Spec is immutable, so fail it.
				return &apis.FieldError{
					Message: "Immutable fields changed (-old +new)",
					Paths:   []string{"spec"},
					Details: diff,
				}
			}
		}
		return nil
	}
	var errs *apis.FieldError
	if b.Spec.Config == nil {
		return apis.ErrMissingField("config").ViaField("spec")
	} else {
		if b.Spec.Config.Namespace == "" {
			errs = errs.Also(apis.ErrMissingField("namespace").ViaField("config").ViaField("spec"))
		}
		if b.Spec.Config.Name == "" {
			errs = errs.Also(apis.ErrMissingField("name").ViaField("config").ViaField("spec"))
		}
		if b.Spec.Config.Kind == "" {
			errs = errs.Also(apis.ErrMissingField("kind").ViaField("config").ViaField("spec"))
		}
		if b.Spec.Config.APIVersion == "" {
			errs = errs.Also(apis.ErrMissingField("apiVersion").ViaField("config").ViaField("spec"))
		}

		// If either APIVersion or Kind is missing, just bail here so we don't print more verbose errors than necessary
		if b.Spec.Config.Kind == "" || b.Spec.Config.APIVersion == "" {
			return errs
		}

		gvk := fmt.Sprintf("%s.%s", b.Spec.Config.Kind, b.Spec.Config.APIVersion)

		switch gvk {
		case "RabbitmqCluster.rabbitmq.com/v1beta1":
		default:
			errs = errs.Also(apis.ErrGeneric("Configuration not supported, only [kind: RabbitmqCluster, apiVersion: rabbitmq.com/v1beta1]")).ViaField("spec").ViaField("config")
		}
	}
	if errs.Error() == "" {
		return nil
	}
	return errs
}

func ValidateBroker(ctx context.Context, unstructured *unstructured.Unstructured) error {
	return validate(ctx, unstructured, &RabbitBroker{})
}

func validate(ctx context.Context, unstructured *unstructured.Unstructured, t apis.Validatable) error {
	if unstructured == nil {
		return nil
	}
	if err := duck.FromUnstructured(unstructured, t); err != nil {
		return err
	}
	err := t.Validate(ctx)
	if err == nil {
		return nil
	}
	return err
}
