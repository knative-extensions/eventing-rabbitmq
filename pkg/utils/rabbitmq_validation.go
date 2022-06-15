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

package utils

import (
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	"knative.dev/pkg/apis"
)

func ValidateRabbitMQClusterReference(cref *v1beta1.RabbitmqClusterReference) *apis.FieldError {
	if cref == nil {
		return apis.ErrMissingField("rabbitmqClusterReference").ViaField("spec")
	} else {
		if cref.Name == "" {
			if cref.ConnectionSecret == nil {
				return apis.ErrMissingField("name").ViaField("rabbitmqClusterReference").ViaField("spec")
			}
		} else if cref.ConnectionSecret != nil {
			return &apis.FieldError{
				Message: "rabbitmqClusterReference.name and rabbitmqClusterReference.connectionSecret must not be set simultaneously",
				Paths:   []string{"spec", "rabbitmqClusterReference", "connectionSecret"},
			}
		}
	}

	return nil
}
