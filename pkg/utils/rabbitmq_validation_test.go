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

package utils

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

func TestValidateRabbitMQClusterReference(t *testing.T) {
	testCases := map[string]struct {
		cRef    *v1beta1.RabbitmqClusterReference
		wantErr bool
	}{
		"missing rabbitmqClusterReference.name": {
			cRef:    &v1beta1.RabbitmqClusterReference{Namespace: "test"},
			wantErr: true,
		},
		"including connectionSecret": {
			cRef: &v1beta1.RabbitmqClusterReference{
				Namespace: "test", Name: "test", ConnectionSecret: &v1.LocalObjectReference{Name: "test"}},
			wantErr: true,
		},
		"just connection secret": {
			cRef:    &v1beta1.RabbitmqClusterReference{ConnectionSecret: &v1.LocalObjectReference{Name: "test"}},
			wantErr: false,
		},
		"just connection secret and namespace": {
			cRef: &v1beta1.RabbitmqClusterReference{
				Namespace: "test", ConnectionSecret: &v1.LocalObjectReference{Name: "test"}},
			wantErr: false,
		},
		"name and namespace": {
			cRef: &v1beta1.RabbitmqClusterReference{
				Namespace: "test", Name: "test"},
			wantErr: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			err := ValidateRabbitMQClusterReference(tc.cRef)
			if err == nil && tc.wantErr {
				t.Error("expected error but got nil")
			} else if err != nil && !tc.wantErr {
				t.Errorf("got error %s, when not expecting error", err.Error())
			}
		})
	}
}
