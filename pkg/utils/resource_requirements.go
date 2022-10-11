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
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	CPURequestAnnotation    = "rabbitmq.eventing.knative.dev/cpu-request"
	CPULimitAnnotation      = "rabbitmq.eventing.knative.dev/cpu-limit"
	MemoryRequestAnnotation = "rabbitmq.eventing.knative.dev/memory-request"
	MemoryLimitAnnotation   = "rabbitmq.eventing.knative.dev/memory-limit"
)

func ValidateResourceRequestsAndLimits(obj metav1.ObjectMeta) *apis.FieldError {
	cpuRequest, apiErr := parseResource(obj, CPURequestAnnotation)
	if apiErr != nil {
		return apiErr
	}
	cpuLimit, apiErr := parseResource(obj, CPULimitAnnotation)
	if apiErr != nil {
		return apiErr
	}
	if cpuRequest != nil && cpuLimit != nil {
		if cpuRequest.Cmp(*cpuLimit) > 0 {
			return &apis.FieldError{
				Message: "request must be less than or equal to limit",
				Paths:   []string{"metadata", "annotations"},
			}
		}
	}

	memRequest, apiErr := parseResource(obj, MemoryRequestAnnotation)
	if apiErr != nil {
		return apiErr
	}
	memLimit, apiErr := parseResource(obj, MemoryLimitAnnotation)
	if apiErr != nil {
		return apiErr
	}
	if memRequest != nil && memLimit != nil {
		if memRequest.Cmp(*memLimit) > 0 {
			return &apis.FieldError{
				Message: "request must be less than or equal to limit",
				Paths:   []string{"metadata", "annotations"},
			}
		}
	}
	return nil
}

func GetResourceRequirements(obj metav1.ObjectMeta) (corev1.ResourceRequirements, error) {
	requirements := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
	if apiErr := ValidateResourceRequestsAndLimits(obj); apiErr != nil {
		return requirements, errors.New(apiErr.Details)
	}

	// No need to check for errors as the validation happens above
	if cpuRequest, _ := parseResource(obj, CPURequestAnnotation); cpuRequest != nil {
		requirements.Requests[corev1.ResourceCPU] = *cpuRequest
	}
	if cpuLimit, _ := parseResource(obj, CPULimitAnnotation); cpuLimit != nil {
		requirements.Limits[corev1.ResourceCPU] = *cpuLimit
	}
	if memRequest, _ := parseResource(obj, MemoryRequestAnnotation); memRequest != nil {
		requirements.Requests[corev1.ResourceMemory] = *memRequest
	}
	if memLimit, _ := parseResource(obj, MemoryLimitAnnotation); memLimit != nil {
		requirements.Limits[corev1.ResourceMemory] = *memLimit
	}

	return requirements, nil
}

func parseResource(obj metav1.ObjectMeta, annotation string) (*resource.Quantity, *apis.FieldError) {
	quantity, ok := obj.GetAnnotations()[annotation]
	if ok {
		q, err := resource.ParseQuantity(quantity)
		if err != nil {
			return nil, &apis.FieldError{
				Message: fmt.Sprintf("Failed to parse quantity from %s", annotation),
				Paths:   []string{"metadata", "annotations", annotation},
				Details: err.Error(),
			}
		}
		return &q, nil
	}

	return nil, nil
}
