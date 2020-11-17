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

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"knative.dev/eventing-rabbitmq/test/e2e/config/smoke/brokertrigger"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
)

const (
	interval = 1 * time.Second
	timeout  = 5 * time.Minute
)

// SmokeTestBrokerImpl makes sure an RabbitMQ Broker goes ready.
func SmokeTestBrokerTrigger() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install a broker", brokertrigger.Install())
	f.Alpha("RabbitMQ broker").Must("goes ready", AllGoReady)
	return f
}

func AllGoReady(ctx context.Context, t *testing.T) {
	env := environment.FromContext(ctx)
	for _, ref := range env.References() {
		if !strings.Contains(ref.APIVersion, "knative.dev") {
			// Let's not care so much about checking the status of non-Knative
			// resources.
			continue
		}
		if err := k8s.WaitForReadyOrDone(ctx, ref, interval, timeout); err != nil {
			t.Fatal("failed to wait for ready or done, ", err)
		}
	}
	t.Log("all resources ready")
}
