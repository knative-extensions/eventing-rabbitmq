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

package main

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
)

var ourTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	v1alpha1.SchemeGroupVersion.WithKind("RabbitmqSource"): &v1alpha1.RabbitmqSource{},
}

var callbacks = map[schema.GroupVersionKind]validation.Callback{}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"webhook.rabbitmq.sources.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		ourTypes,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}
	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.rabbitmq.sources.knative.dev",

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

func NewConfigValidationController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	return configmaps.NewAdmissionController(ctx,
		// Name of the configmap webhook.
		"config.webhook.rabbitmq.sources.knative.dev",

		// The path on which to serve the webhook.
		"/config-validation",

		// The configmaps to validate.
		configmap.Constructors{
			o11yconfigmap.Name():           o11yconfigmap.Parse,
			logging.ConfigMapName():        logging.NewConfigFromConfigMap,
			leaderelection.ConfigMapName(): leaderelection.NewConfigFromConfigMap,
		},
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "rabbitmq-webhook",
		Port:        webhook.PortFromEnv(8443),
		// SecretName must match the name of the Secret created in the configuration.
		SecretName: "rabbitmq-webhook-certs",
	})

	sharedmain.WebhookMainWithContext(ctx, "rabbitmq-webhook",
		certificates.NewController,
		NewConfigValidationController,
		NewValidationAdmissionController,
		NewDefaultingAdmissionController,
	)
}
