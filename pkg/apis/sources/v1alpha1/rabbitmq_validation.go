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

package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

func (current *RabbitmqSource) Validate(ctx context.Context) *apis.FieldError {
	if current.Spec.RabbitmqResourcesConfig == nil {
		return apis.ErrMissingField("rabbitmqResourcesConfig").ViaField("spec")
	} else {
		if !current.Spec.RabbitmqResourcesConfig.Predeclared && current.Spec.RabbitmqResourcesConfig.ExchangeName == "" {
			return &apis.FieldError{
				Message: "Name of exchange must be provided when spec.predeclared is false",
				Paths:   []string{"spec", "rabbitmqResourcesConfig", "name"},
			}
		}
	}

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*RabbitmqSource)
		if diff, err := kmp.ShortDiff(original.Spec, current.Spec); err != nil {
			return &apis.FieldError{
				Message: "Failed to diff RabbitmqSource",
				Paths:   []string{"spec"},
				Details: err.Error(),
			}
		} else if diff != "" {
			return &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"spec"},
				Details: diff,
			}
		}
	}

	if current.Spec.RabbitmqClusterReference == nil {
		return apis.ErrMissingField("rabbitmqClusterReference").ViaField("spec")
	} else {
		if current.Spec.RabbitmqClusterReference.Name == "" {
			if current.Spec.RabbitmqClusterReference.ConnectionSecret == nil {
				return apis.ErrMissingField("name").ViaField("rabbitmqClusterReference").ViaField("spec")
			}
		} else if current.Spec.RabbitmqClusterReference.ConnectionSecret != nil {
			return apis.ErrDisallowedFields("connectionSecret").ViaField("rabbitmqClusterReference").ViaField("spec")
		}
	}

	return current.Spec.RabbitmqResourcesConfig.validate().ViaField("rabbitmqResourcesConfig")
}

func (rmqResSpec *RabbitmqResourcesConfigSpec) validate() *apis.FieldError {
	if rmqResSpec.Parallelism == nil {
		return nil
	}

	if *rmqResSpec.Parallelism < 1 || *rmqResSpec.Parallelism > 1000 {
		return apis.ErrOutOfBoundsValue(*rmqResSpec.Parallelism, 1, 1000, "Parallelism")
	}

	return nil
}
