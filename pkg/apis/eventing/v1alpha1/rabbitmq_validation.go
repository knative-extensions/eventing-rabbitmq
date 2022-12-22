/*
Copyright 2022 The Knative Authors

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

	"knative.dev/eventing-rabbitmq/pkg/utils"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

func (r *RabbitmqBrokerConfig) Validate(ctx context.Context) *apis.FieldError {
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*RabbitmqBrokerConfig)
		if original != nil {
			if diff, err := kmp.ShortDiff(original.Spec, r.Spec); err != nil {
				return &apis.FieldError{
					Message: "Failed to diff RabbitmqBrokerConfig",
					Paths:   []string{"spec"},
					Details: err.Error(),
				}
			} else if diff != "" {
				// spec is immutable
				return &apis.FieldError{
					Message: "Immutable fields changed",
					Paths:   []string{"spec"},
					Details: diff,
				}
			}
		}
	}

	if apiErr := utils.ValidateRabbitMQClusterReference(r.Spec.RabbitmqClusterReference); apiErr != nil {
		return apiErr
	}

	return nil
}
