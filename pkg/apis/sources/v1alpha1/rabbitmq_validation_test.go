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

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	defaultParallelism = 1
	fullSpec           = RabbitmqSourceSpec{
		Brokers: "amqp://guest:guest@localhost:5672/",
		Topic:   "logs_topic",
		ExchangeConfig: RabbitmqSourceExchangeConfigSpec{
			Name:       "an-exchange",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
		},
		QueueConfig: RabbitmqSourceQueueConfigSpec{
			Name:       "",
			RoutingKey: "*.critical",
			Durable:    false,
			AutoDelete: false,
		},
		ChannelConfig: RabbitmqChannelConfigSpec{
			Parallelism: &defaultParallelism,
			GlobalQos:   false,
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
		"Topic changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Topic:              "some-other-topic",
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Brokers changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Brokers:            "broker1",
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.APIVersion changed": {
			orig: &fullSpec,
			updated: RabbitmqSourceSpec{
				Topic: fullSpec.Topic,
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
				Topic: fullSpec.Topic,
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
				Topic: fullSpec.Topic,
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
				Topic: fullSpec.Topic,
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
				Topic: fullSpec.Topic,
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
				Brokers:        fullSpec.Brokers,
				Topic:          fullSpec.Topic,
				ExchangeConfig: fullSpec.ExchangeConfig,
				QueueConfig: RabbitmqSourceQueueConfigSpec{
					Name:       "",
					RoutingKey: "*.critical",
					Durable:    false,
					AutoDelete: false,
				},
				ChannelConfig:      fullSpec.ChannelConfig,
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
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
					Spec: *tc.spec,
				}

				var err *apis.FieldError
				if tc.isInUpdate {
					updated := &RabbitmqSource{
						Spec: *tc.spec,
					}
					updated.Spec.ChannelConfig = RabbitmqChannelConfigSpec{
						Parallelism: &tc.parallelism,
					}
					ctx = apis.WithinUpdate(ctx, orig)
					err = updated.Validate(ctx)
				} else {
					orig.Spec.ChannelConfig = RabbitmqChannelConfigSpec{
						Parallelism: &tc.parallelism,
					}

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
		spec                 *RabbitmqSourceExchangeConfigSpec
		predeclared, allowed bool
	}{
		"not allowed when predeclared set to false and no exchange name set": {
			spec:        &RabbitmqSourceExchangeConfigSpec{},
			predeclared: false,
			allowed:     false,
		},
		"allowed when predeclared set to true and no exchange name set": {
			spec:        &RabbitmqSourceExchangeConfigSpec{},
			predeclared: true,
			allowed:     true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			src := &RabbitmqSource{
				Spec: RabbitmqSourceSpec{
					ExchangeConfig: *tc.spec,
					Predeclared:    tc.predeclared,
				},
			}

			err := src.Validate(context.TODO())
			if tc.allowed != (err == nil) {
				t.Fatalf("ExchangeConfig validation result incorrect. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
