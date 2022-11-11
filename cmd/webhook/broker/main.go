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

package main

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	rabbitv1 "knative.dev/eventing-rabbitmq/pkg/apis/eventing/v1alpha1"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

var ourTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	v1.SchemeGroupVersion.WithKind("Broker"):                                                             &rabbitv1.RabbitBroker{},
	v1.SchemeGroupVersion.WithKind("Trigger"):                                                            &v1.Trigger{},
	schema.GroupVersion{Group: eventing.GroupName, Version: "v1alpha1"}.WithKind("RabbitmqBrokerConfig"): &rabbitv1.RabbitmqBrokerConfig{},
}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"defaulting.webhook.rabbitmq.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
			schema.GroupVersion{Group: eventing.GroupName, Version: "v1alpha1"}.WithKind("RabbitmqBrokerConfig"): &rabbitv1.RabbitmqBrokerConfig{},
		},

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
	featureStore.WatchConfigs(cmw)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return featureStore.ToContext(ctx)
	}

	callbacks := map[schema.GroupVersionKind]validation.Callback{
		v1.SchemeGroupVersion.WithKind("Broker"):               validation.NewCallback(rabbitv1.ValidateBroker, webhook.Create, webhook.Update),
		v1.SchemeGroupVersion.WithKind("Trigger"):              validation.NewCallback(rabbitv1.ValidateTrigger(ctx), webhook.Create, webhook.Update),
		v1.SchemeGroupVersion.WithKind("RabbitmqBrokerConfig"): validation.NewCallback(rabbitv1.ValidateRabbitmqBrokerConfig, webhook.Create, webhook.Update),
	}
	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.rabbitmq.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate.
		ourTypes,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,

		// Extra validating callbacks to be applied to resources.
		callbacks,
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "rabbitmq-broker-webhook",
		Port:        webhook.PortFromEnv(8443),
		// SecretName must match the name of the Secret created in the configuration.
		SecretName: "rabbitmq-broker-webhook-certs",
	})

	sharedmain.WebhookMainWithContext(ctx, "rabbitmq-broker-webhook",
		certificates.NewController,
		NewValidationAdmissionController,
		NewDefaultingAdmissionController,
	)
}
