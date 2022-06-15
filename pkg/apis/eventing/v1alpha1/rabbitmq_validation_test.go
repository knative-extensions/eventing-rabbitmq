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
	"testing"

	v1 "k8s.io/api/core/v1"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	"knative.dev/pkg/apis"
)

func TestValidateRabbitmqBrokerConfig(t *testing.T) {
	validBrokerConfig := &RabbitmqBrokerConfig{
		Spec: RabbitmqBrokerConfigSpec{
			QueueType: "quorum",
			RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
				Namespace: "some-namespace",
				Name:      "some-name",
			},
		},
	}

	tests := map[string]struct {
		newConfig *RabbitmqBrokerConfig
		wantErr   bool
		isUpdate  bool
	}{
		"no op update": {
			newConfig: validBrokerConfig,
			wantErr:   false,
			isUpdate:  true,
		},
		"spec update": {
			newConfig: &RabbitmqBrokerConfig{
				Spec: RabbitmqBrokerConfigSpec{
					RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
						Namespace: "new-namespace",
						Name:      "some-name",
					},
				},
			},
			wantErr:  true,
			isUpdate: true,
		},
		"missing rabbitmqClusterReference": {
			newConfig: &RabbitmqBrokerConfig{
				Spec: RabbitmqBrokerConfigSpec{
					QueueType: "classic",
				},
			},
			wantErr:  true,
			isUpdate: false,
		},
		"missing rabbitmqClusterReference.name": {
			newConfig: &RabbitmqBrokerConfig{
				Spec: RabbitmqBrokerConfigSpec{
					RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
						Namespace: "some-namespace",
					},
				},
			},
			wantErr:  true,
			isUpdate: false,
		},
		"with rabbitmqClusterReference.namespace": {
			newConfig: &RabbitmqBrokerConfig{
				Spec: RabbitmqBrokerConfigSpec{
					RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
						Name: "some-name", Namespace: "test",
					},
				},
			},
			wantErr:  false,
			isUpdate: false,
		},
		"including connectionSecret": {
			newConfig: &RabbitmqBrokerConfig{
				Spec: RabbitmqBrokerConfigSpec{
					RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
						Name:      "some-name",
						Namespace: "some-namespace",
						ConnectionSecret: &v1.LocalObjectReference{
							Name: "some-secret",
						},
					},
				},
			},
			wantErr:  true,
			isUpdate: false,
		},
		"just connectionSecret": {
			newConfig: &RabbitmqBrokerConfig{
				Spec: RabbitmqBrokerConfigSpec{
					RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
						Namespace: "some-namespace",
						ConnectionSecret: &v1.LocalObjectReference{
							Name: "some-secret",
						},
					},
				},
			},
			wantErr:  false,
			isUpdate: false,
		},
	}

	for n, test := range tests {
		t.Run(n, func(t *testing.T) {
			ctx := context.TODO()
			if test.isUpdate {
				ctx = apis.WithinUpdate(ctx, validBrokerConfig)
			} else {
				ctx = apis.WithinCreate(ctx)
			}

			got := test.newConfig.Validate(ctx)
			if got == nil && test.wantErr {
				t.Error("expected error but got nil")
			} else if got != nil && !test.wantErr {
				t.Errorf("got error %s, when not expecting error", got.Error())
			}
		})
	}
}

func TestValidateRabbitmqBrokerConfig_Create(t *testing.T) {

}
