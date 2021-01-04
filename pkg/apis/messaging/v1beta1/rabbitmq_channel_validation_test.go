/*
Copyright 2019 The Knative Authors

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

package v1beta1

import (
	"context"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"testing"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/webhook/resourcesemantics"

	"knative.dev/pkg/apis"
)

func TestRabbitmqChannelValidation(t *testing.T) {
	aURL, _ := apis.ParseURL("http://example.com")

	validConfig := duckv1.KReference{
		Kind: "Secret",
		APIVersion: "v1",
		Name: "secretname",
		Namespace: "secretns",
	}

	testCases := map[string]struct {
		cr   resourcesemantics.GenericCRD
		want *apis.FieldError
	}{
		"empty spec": {
			cr: &RabbitmqChannel{
				Spec: RabbitmqChannelSpec{},
			},
			want: apis.ErrMissingField("spec.config.apiVersion", "spec.config.kind", "spec.config.name"),
		},
		"invalid config unknown kind": {
			cr: &RabbitmqChannel{
				Spec: RabbitmqChannelSpec{
					Config: duckv1.KReference{
						Kind: "badkind",
						APIVersion: "v1",
						Name: "doesnotmatter",
					},
				},
			},
			want: func() *apis.FieldError {
				fe := &apis.FieldError{
					Message: "unsupported config",
					Paths:   []string{"spec.config.apiVersion", "spec.config.kind"},
				}
				fe.Details = `unsupported config must be either Secret.v1 or RabbitmqCluster.rabbitmq.com/v1beta1, was "badkind.v1"`
				return fe
			}(),
		},
		"invalid config, wrong version for secret": {
			cr: &RabbitmqChannel{
				Spec: RabbitmqChannelSpec{
					Config: duckv1.KReference{
						Kind: "Secret",
						APIVersion: "v1beta1",
						Name: "doesnotmatter",
					},
				},
			},
			want: func() *apis.FieldError {
				fe := &apis.FieldError{
					Message: "unsupported config",
					Paths:   []string{"spec.config.apiVersion", "spec.config.kind"},
				}
				fe.Details = `unsupported config must be either Secret.v1 or RabbitmqCluster.rabbitmq.com/v1beta1, was "Secret.v1beta1"`
				return fe
			}(),
		},
		"invalid config, wrong version for rabbitmqcluster": {
			cr: &RabbitmqChannel{
				Spec: RabbitmqChannelSpec{
					Config: duckv1.KReference{
						Kind: "RabbitmqCluster",
						APIVersion: "rabbitmq.com/v1alpha1",
						Name: "doesnotmatter",
					},
				},
			},
			want: func() *apis.FieldError {
				fe := &apis.FieldError{
					Message: "unsupported config",
					Paths:   []string{"spec.config.apiVersion", "spec.config.kind"},
				}
				fe.Details = `unsupported config must be either Secret.v1 or RabbitmqCluster.rabbitmq.com/v1beta1, was "RabbitmqCluster.rabbitmq.com/v1alpha1"`
				return fe
			}(),
		},
		"valid config with rabbitmqcluster": {
			cr: &RabbitmqChannel{
				Spec: RabbitmqChannelSpec{
					Config: duckv1.KReference{
						Kind: "RabbitmqCluster",
						APIVersion: "rabbitmq.com/v1beta1",
						Name: "doesnotmatter",
					},
				},
			},
			want: nil,
		},
		"valid subscribers array": {
			cr: &RabbitmqChannel{
				Spec: RabbitmqChannelSpec{
					Config: validConfig,
					ChannelableSpec: eventingduckv1.ChannelableSpec{
						SubscribableSpec: eventingduckv1.SubscribableSpec{
							Subscribers: []eventingduckv1.SubscriberSpec{{
								SubscriberURI: aURL,
								ReplyURI:      aURL,
							}},
						},
					},
				},
			},
			want: nil,
		},
		"empty subscriber at index 1": {
			cr: &RabbitmqChannel{
				Spec: RabbitmqChannelSpec{
					Config: validConfig,
					ChannelableSpec: eventingduckv1.ChannelableSpec{
						SubscribableSpec: eventingduckv1.SubscribableSpec{
							Subscribers: []eventingduckv1.SubscriberSpec{{
								SubscriberURI: aURL,
								ReplyURI:      aURL,
							}, {}},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				fe := apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
				fe.Details = "expected at least one of, got none"
				return fe
			}(),
		},
		"two empty subscribers": {
			cr: &RabbitmqChannel{
				Spec: RabbitmqChannelSpec{
					Config: validConfig,
					ChannelableSpec: eventingduckv1.ChannelableSpec{
						SubscribableSpec: eventingduckv1.SubscribableSpec{
							Subscribers: []eventingduckv1.SubscriberSpec{{}, {}},
						},
					},
				},
			},
			want: func() *apis.FieldError {
				var errs *apis.FieldError
				fe := apis.ErrMissingField("spec.subscribable.subscriber[0].replyURI", "spec.subscribable.subscriber[0].subscriberURI")
				fe.Details = "expected at least one of, got none"
				errs = errs.Also(fe)
				fe = apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
				fe.Details = "expected at least one of, got none"
				errs = errs.Also(fe)
				return errs
			}(),
		},
	}

	for n, test := range testCases {
		t.Run(n, func(t *testing.T) {
			got := test.cr.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validate (-want, +got) = %v", n, diff)
			}
		})
	}
}
