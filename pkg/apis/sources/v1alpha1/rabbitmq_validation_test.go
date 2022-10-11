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
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	defaultParallelism = 1
	fullSpec           = RabbitmqSourceSpec{
		RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
			Name:      "test-cluster",
			Namespace: "test",
		},
		RabbitmqResourcesConfig: &RabbitmqResourcesConfigSpec{
			ExchangeName: "an-exchange",
			QueueName:    "",
			Parallelism:  &defaultParallelism,
		},
		Sink: &duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: "foo",
				Kind:       "bar",
				Namespace:  "baz",
				Name:       "qux",
			},
		},
		ServiceAccountName: "service-account-name",
	}
)

func TestRabbitmqSourceCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    *RabbitmqSourceSpec
		updated RabbitmqSourceSpec
		allowed bool
	}{
		"nil orig": {
			updated: fullSpec,
			allowed: true,
		},
		"Broker changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.APIVersion changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Sink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "some-other-api-version",
						Kind:       fullSpec.Sink.Ref.APIVersion,
						Namespace:  fullSpec.Sink.Ref.Namespace,
						Name:       fullSpec.Sink.Ref.Name,
					},
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Kind changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Sink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: fullSpec.Sink.Ref.APIVersion,
						Kind:       "some-other-kind",
						Namespace:  fullSpec.Sink.Ref.Namespace,
						Name:       fullSpec.Sink.Ref.Name,
					},
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Namespace changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Sink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: fullSpec.Sink.Ref.APIVersion,
						Kind:       fullSpec.Sink.Ref.Kind,
						Namespace:  "some-other-namespace",
						Name:       fullSpec.Sink.Ref.Name,
					},
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Name changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Sink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: fullSpec.Sink.Ref.APIVersion,
						Kind:       fullSpec.Sink.Ref.Kind,
						Namespace:  fullSpec.Sink.Ref.Namespace,
						Name:       "some-other-name",
					},
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"ServiceAccountName changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Sink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: fullSpec.Sink.Ref.APIVersion,
						Kind:       fullSpec.Sink.Ref.Kind,
						Namespace:  fullSpec.Sink.Ref.Namespace,
						Name:       "some-other-name",
					},
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"no change": {
			orig:    &fullSpec,
			updated: fullSpec,
			allowed: true,
		},
		"removed cluster reference": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				RabbitmqClusterReference: nil,
			},
			allowed: false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx := context.TODO()
			if tc.orig != nil {
				orig := &RabbitmqSource{
					Spec: *tc.orig,
				}

				ctx = apis.WithinUpdate(ctx, orig)
			}
			updated := &RabbitmqSource{
				Spec: tc.updated,
			}
			err := updated.Validate(ctx)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}

func TestRabbitmqSourceCheckChannelParallelismValue(t *testing.T) {
	testCases := map[string]struct {
		spec                *RabbitmqSourceSpec
		parallelism         int
		allowed, isInUpdate bool
	}{
		"nil spec": {
			spec:    nil,
			allowed: true,
		},
		"valid parallelism value": {
			spec:        &fullSpec,
			parallelism: 1,
			allowed:     true,
		},
		"negative parallelism value in spec": {
			spec:        &fullSpec,
			parallelism: -1,
			allowed:     false,
		},
		"out of bounds parallelism value in spec": {
			spec:        &fullSpec,
			parallelism: 1001,
			allowed:     false,
		},
		"zero parallelism value in spec on update": {
			spec:        &fullSpec,
			parallelism: 0,
			allowed:     false,
		},
		"out of bounds parallelism value in spec on update": {
			spec:        &fullSpec,
			parallelism: 1001,
			allowed:     false,
		},
		"valid channel parallelism value update": {
			spec: &RabbitmqSourceSpec{
				RabbitmqClusterReference: fullSpec.RabbitmqClusterReference,
				RabbitmqResourcesConfig:  fullSpec.RabbitmqResourcesConfig,
				Sink:                     fullSpec.Sink,
				ServiceAccountName:       fullSpec.ServiceAccountName,
			},
			parallelism: 102,
			allowed:     true,
			isInUpdate:  true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx := context.TODO()
			if tc.spec != nil {
				orig := &RabbitmqSource{
					Spec: *tc.spec.DeepCopy(),
				}

				var err *apis.FieldError
				if tc.isInUpdate {
					updated := &RabbitmqSource{
						Spec: orig.Spec,
					}
					updated.Spec.RabbitmqResourcesConfig.Parallelism = &tc.parallelism
					ctx = apis.WithinUpdate(ctx, orig)
					err = updated.Validate(ctx)
				} else {
					orig.Spec.RabbitmqResourcesConfig.Parallelism = &tc.parallelism
					ctx = apis.WithinCreate(ctx)
					err = orig.Validate(ctx)
				}

				if tc.allowed != (err == nil) {
					t.Fatalf("Unexpected parallelism value check. Expected %v. Actual %v", tc.allowed, err)
				}
			}
		})
	}
}

func TestRabbitmqSourceExchangeConfig(t *testing.T) {
	testCases := map[string]struct {
		predeclared, allowed bool
	}{
		"not allowed when predeclared set to false and no exchange name set": {
			predeclared: false,
			allowed:     false,
		},
		"allowed when predeclared set to true and no exchange name set": {
			predeclared: true,
			allowed:     true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			src := &RabbitmqSource{
				Spec: RabbitmqSourceSpec{
					RabbitmqResourcesConfig: &RabbitmqResourcesConfigSpec{
						Predeclared: tc.predeclared,
					},
					RabbitmqClusterReference: fullSpec.RabbitmqClusterReference,
				},
			}

			err := src.Validate(context.TODO())
			if tc.allowed != (err == nil) {
				t.Fatalf("rabbitmqResourcesConfig validation result incorrect. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}

func TestRabbitmqSourceValidation(t *testing.T) {
	testCases := map[string]struct {
		spec    *RabbitmqSourceSpec
		meta    metav1.ObjectMeta
		wantErr bool
	}{
		"valid config": {
			spec:    &fullSpec,
			wantErr: false,
		},
		"missing rabbitmqClusterReference": {
			spec: &RabbitmqSourceSpec{
				RabbitmqResourcesConfig: &RabbitmqResourcesConfigSpec{Predeclared: true},
			},
			wantErr: true,
		},
		"missing rabbitmqResourcesConfig": {
			spec: &RabbitmqSourceSpec{
				RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{Namespace: "test"},
			},
			wantErr: true,
		},
		"missing rabbitmqClusterReference.name": {
			spec: &RabbitmqSourceSpec{
				RabbitmqResourcesConfig:  &RabbitmqResourcesConfigSpec{Predeclared: true},
				RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{Namespace: "test"},
			},
			wantErr: true,
		},
		"including connectionSecret": {
			spec: &RabbitmqSourceSpec{
				RabbitmqResourcesConfig: &RabbitmqResourcesConfigSpec{Predeclared: true},
				RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
					Namespace: "test", Name: "test", ConnectionSecret: &v1.LocalObjectReference{Name: "test"}},
			},
			wantErr: true,
		},
		"just connection secret": {
			spec: &RabbitmqSourceSpec{
				RabbitmqResourcesConfig: &RabbitmqResourcesConfigSpec{Predeclared: true},
				RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
					Namespace: "test", ConnectionSecret: &v1.LocalObjectReference{Name: "test"}},
			},
			wantErr: false,
		},
		"name and namespace": {
			spec: &RabbitmqSourceSpec{
				RabbitmqResourcesConfig: &RabbitmqResourcesConfigSpec{Predeclared: true},
				RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{
					Namespace: "test", Name: "test"},
			},
			wantErr: false,
		},
		"invalid resource requirements": {
			spec: &fullSpec,
			meta: metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.CPURequestAnnotation: "invalid",
				},
			},
			wantErr: true,
		},
		"valid resource requirements": {
			spec: &fullSpec,
			meta: metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.CPURequestAnnotation: "50m",
				},
			},
			wantErr: false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			src := &RabbitmqSource{
				ObjectMeta: tc.meta,
				Spec:       *tc.spec,
			}

			err := src.Validate(context.TODO())
			if err == nil && tc.wantErr {
				t.Error("expected error but got nil")
			} else if err != nil && !tc.wantErr {
				t.Errorf("got error %s, when not expecting error", err.Error())
			}
		})
	}
}
