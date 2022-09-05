//go:build e2e
// +build e2e

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

package rekt

import (
	"testing"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing-rabbitmq/test/conformance/features/rabbitmqcluster"
	r "knative.dev/eventing-rabbitmq/test/conformance/resources/rabbitmqcluster"
	"knative.dev/eventing/test/rekt/features/broker"
	b "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
)

// TestBrokerConformance
func TestBrokerControlPlaneConformance(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Prerequisite(ctx, t, rabbitmqcluster.GoesReady("rabbitbroker", r.WithEnvConfig()...))
	// Install and wait for a Ready Broker.
	env.Prerequisite(ctx, t, broker.GoesReady("default", b.WithEnvConfig()...))
	env.TestSet(ctx, t, broker.ControlPlaneConformance("default"))
}

func TestBrokerDataPlaneConformance(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Prerequisite(ctx, t, rabbitmqcluster.GoesReady("rabbitbroker", r.WithEnvConfig()...))
	// Install and wait for a Ready Broker.
	env.Prerequisite(ctx, t, broker.GoesReady("default3", b.WithEnvConfig()...))

	env.TestSet(ctx, t, broker.DataPlaneConformance("default3"))
}
