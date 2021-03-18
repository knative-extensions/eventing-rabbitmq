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

	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing/test/rekt/features/broker"
	b "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/reconciler-test/pkg/environment"
)

// TestBrokerConformance
func TestBrokerConformance(t *testing.T) {
	ctx, env := global.Environment(environment.Managed(t))
	cfg := []b.CfgFn{b.WithBrokerClass(eventingGlobal.BrokerClass)}
	if eventingGlobal.BrokerTemplatesDir != "" {
		cfg = append(cfg, b.WithBrokerTemplateFiles(eventingGlobal.BrokerTemplatesDir))
	}
	// Install and wait for a Ready Broker.
	env.Prerequisite(ctx, t, broker.GoesReady("default", cfg...))
	env.TestSet(ctx, t, broker.ControlPlaneConformance("default"))
	env.TestSet(ctx, t, broker.DataPlaneConformance("default"))
}
