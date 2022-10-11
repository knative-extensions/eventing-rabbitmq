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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateResourceRequestsAndLimits(t *testing.T) {
	testCases := map[string]struct {
		obj     metav1.ObjectMeta
		wantErr bool
	}{
		"empty meta should not cause error": {
			obj:     metav1.ObjectMeta{},
			wantErr: false,
		},
		"CPU request bad format": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					CPURequestAnnotation: "invalid",
				},
			},
			wantErr: true,
		},
		"CPU limit bad format": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					CPULimitAnnotation: "invalid",
				},
			},
			wantErr: true,
		},
		"Memory request bad format": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					MemoryRequestAnnotation: "invalid",
				},
			},
			wantErr: true,
		},
		"Memory limit bad format": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					MemoryLimitAnnotation: "invalid",
				},
			},
			wantErr: true,
		},
		"CPU request is greater than limit": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					CPURequestAnnotation: "50m",
					CPULimitAnnotation:   "25m",
				},
			},
			wantErr: true,
		},
		"Memory request is greater than limit": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					MemoryRequestAnnotation: "50M",
					MemoryLimitAnnotation:   "25M",
				},
			},
			wantErr: true,
		},
		"no error when all set correctly": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					CPURequestAnnotation:    "25m",
					CPULimitAnnotation:      "50m",
					MemoryRequestAnnotation: "50M",
					MemoryLimitAnnotation:   "50M",
				},
			},
			wantErr: false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			err := ValidateResourceRequestsAndLimits(tc.obj)
			if err == nil && tc.wantErr {
				t.Error("expected error but got nil")
			} else if err != nil && !tc.wantErr {
				t.Errorf("got error %s, when not expecting error", err.Error())
			}
		})
	}
}

func TestGetResourceRequirements(t *testing.T) {
	testCases := map[string]struct {
		obj                  metav1.ObjectMeta
		wantErr              bool
		expectedRequirements corev1.ResourceRequirements
	}{
		"returns error when validation fails": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					CPURequestAnnotation: "invalid",
				},
			},
			wantErr: true,
		},
		"parses resources and requests properly. no error": {
			obj: metav1.ObjectMeta{
				Annotations: map[string]string{
					CPURequestAnnotation:    "25m",
					CPULimitAnnotation:      "50m",
					MemoryRequestAnnotation: "50M",
					MemoryLimitAnnotation:   "50M",
				},
			},
			wantErr: false,
			expectedRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("25m"),
					corev1.ResourceMemory: resource.MustParse("50M"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("50M"),
				},
			},
		},
		"empty resource requirements when annotations don't exist. no error": {
			obj:     metav1.ObjectMeta{},
			wantErr: false,
			expectedRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
				Limits:   corev1.ResourceList{},
			},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			actualRequirements, err := GetResourceRequirements(tc.obj)
			if err == nil && tc.wantErr {
				t.Error("expected error but got nil")
			} else if err != nil && !tc.wantErr {
				t.Errorf("got error %s, when not expecting error", err.Error())
			}

			if !tc.wantErr {
				if !reflect.DeepEqual(actualRequirements, tc.expectedRequirements) {
					t.Errorf("actual and expected requirements not equal. Actual: %v, Expected: %v", actualRequirements, tc.expectedRequirements)
				}
			}
		})
	}
}
