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

	"knative.dev/eventing-rabbitmq/test/e2e/config/source"
	"knative.dev/eventing-rabbitmq/test/e2e/config/sourceproducer"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
)

// ConcurrentAdapterProcessingTest verifies that the Adapter sends events
// concurrently. It does this by sending 5 events to an event recorder that
// takes 1 second to respond to each event. It waits for both events to be
// received and then calculates the time between events to determine that the
// adapter did not block on receiving a response to the first event before
// processing the second.
func ConcurrentAdapterProcessingTest() *feature.Feature {
	eventsNumber := 2
	f := new(feature.Feature)

	f.Setup("install test resources", source.Install())
	f.Setup("install recorder", eventshub.Install("recorder", eventshub.StartReceiver, eventshub.ResponseWaitTime(time.Second)))

	f.Requirement("recorder is addressable", k8s.IsAddressable(serviceGVR, "recorder"))
	f.Requirement("RabbitMQ broker goes ready", AllGoReady)

	f.Setup("install producer", sourceproducer.Install(eventsNumber))
	f.Assert("the adapter sends events concurrently", func(ctx context.Context, t feature.T) {
		events := eventshub.StoreFromContext(ctx, "recorder").AssertExact(t, eventsNumber)
		diff := events[1].Time.Sub(events[0].Time)
		if diff >= time.Second {
			t.Fatalf("expected processing to happen concurrently but were sequential. time elapsed between events: %v", diff)
		}
	})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}
