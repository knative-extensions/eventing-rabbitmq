//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	"time"

	"knative.dev/eventing-rabbitmq/test/e2e/config/brokertrigger"
	brokerresources "knative.dev/eventing/test/rekt/resources/broker"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
)

// TestConcurrentDispatch verifies that the dispatcher sends events
// concurrently. It does this by sending 2 events to an event recorder that
// takes 1 second to respond to each event. It waits for both events to be
// received and then calculates the time between events to determine that the
// dispatcher did not block on receiving a response to the first event before
// dispatching the second.
func ConcurrentDispatchTest() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install test resources", brokertrigger.Install(brokertrigger.Topology{
		Parallelism: 10,
		Triggers: []duckv1.KReference{{
			Kind: "Service",
			Name: "recorder",
		}},
	}))

	f.Setup("RabbitMQ broker goes ready", AllGoReady)

	prober := eventshub.NewProber()
	prober.SetTargetResource(brokerresources.GVR(), "testbroker")
	prober.SenderFullEvents(2)
	f.Setup("install source", prober.SenderInstall("source"))
	f.Requirement("sender is finished", prober.SenderDone("source"))

	f.Assert("the dispatcher sends events concurrently", func(ctx context.Context, t feature.T) {
		events := eventshub.StoreFromContext(ctx, "recorder").AssertExact(t, 2)
		diff := events[1].Time.Sub(events[0].Time)
		if diff >= 3*time.Second {
			t.Fatalf("expected dispatch to happen concurrently but were sequential. time elapsed between events: %v", diff)
		}
	})
	f.Teardown("Delete feature resources", f.DeleteResources)

	return f
}
