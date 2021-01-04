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
	"fmt"
	"knative.dev/eventing/pkg/apis/eventing"

	"knative.dev/pkg/apis"
)

func (c *RabbitmqChannel) Validate(ctx context.Context) *apis.FieldError {
	errs := c.Spec.Validate(ctx).ViaField("spec")

	// Validate annotations
	if c.Annotations != nil {
		if scope, ok := c.Annotations[eventing.ScopeAnnotationKey]; ok {
			if scope != eventing.ScopeNamespace && scope != eventing.ScopeCluster {
				iv := apis.ErrInvalidValue(scope, "")
				iv.Details = "expected either 'cluster' or 'namespace'"
				errs = errs.Also(iv.ViaFieldKey("annotations", eventing.ScopeAnnotationKey).ViaField("metadata"))
			}
		}
	}
	return errs
}

func (cs *RabbitmqChannelSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	errs = errs.Also(cs.Config.Validate(ctx).ViaField("config"))
	// The above catches the empty fields, but we also need to check that they are of the supported types
	gvk := fmt.Sprintf("%s.%s", cs.Config.Kind, cs.Config.APIVersion)
	// Note the "." comparison, since if fields are missing, don't pollute the errors.
	if gvk != "." && gvk != "Secret.v1" && gvk != "RabbitmqCluster.rabbitmq.com/v1beta1" {
		errs = errs.Also(&apis.FieldError{
			Message: "unsupported config",
			Paths:   []string{"config.apiVersion", "config.kind"},
			Details: fmt.Sprintf("unsupported config must be either Secret.v1 or RabbitmqCluster.rabbitmq.com/v1beta1, was %q", gvk),
		})
	}

	for i, subscriber := range cs.Subscribers {
		if subscriber.ReplyURI == nil && subscriber.SubscriberURI == nil {
			fe := apis.ErrMissingField("replyURI", "subscriberURI")
			fe.Details = "expected at least one of, got none"
			errs = errs.Also(fe.ViaField(fmt.Sprintf("subscriber[%d]", i)).ViaField("subscribable"))
		}
	}
	return errs
}
