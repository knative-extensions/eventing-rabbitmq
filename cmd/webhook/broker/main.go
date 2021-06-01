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
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	rabbitv1 "knative.dev/eventing-rabbitmq/pkg/apis/eventing/v1"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

var ourTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	//	v1.SchemeGroupVersion.WithKind("Broker"): &v1.Broker{},
	v1.SchemeGroupVersion.WithKind("Broker"): &rabbitv1.RabbitBroker{},
}

var callbacks = map[schema.GroupVersionKind]validation.Callback{
	v1.SchemeGroupVersion.WithKind("Broker"): validation.NewCallback(rabbitv1.ValidateFunc, webhook.Create, webhook.Update),
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
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
	)
}
