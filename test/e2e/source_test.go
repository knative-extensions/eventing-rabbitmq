//go:build e2e
// +build e2e

/*
Copyright 2022 The Knative Authors

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
	"knative.dev/eventing-rabbitmq/test/e2e/config/sourcesecret"
	"knative.dev/eventing-rabbitmq/test/e2e/config/sourcevhost"
	"knative.dev/eventing-rabbitmq/test/e2e/config/vhostsourceproducer"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "knative.dev/pkg/system/testing"
)

//
// producer ---> rabbitmq --[source]--> recorder
//

// DirectSourceTest makes sure an RabbitMQ Source delivers events to a sink.
func DirectSourceClusterRefNS(setClusterRef bool) *feature.Feature {
	eventsNumber := 10
	f := new(feature.Feature)
	var opts []manifest.CfgFn
	if setClusterRef {
		opts = append(opts, sourcesecret.WithBrokerConfigClusterRefNS(setClusterRef))
	}

	f.Setup("install RabbitMQ source", source.Install(opts...))
	f.Alpha("RabbitMQ source").Must("goes ready", AllGoReady)
	// Note this is a different producer than events hub because it publishes
	// directly to RabbitMQ
	f.Setup("install producer", sourceproducer.Install(sourceproducer.WithProducerCount(eventsNumber)))
	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(t, eventsNumber)
			})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

func DirectSourceTestWithCerts() *feature.Feature {
	eventsNumber := 10
	f := new(feature.Feature)

	f.Setup("install RabbitMQ source", source.Install())
	f.Alpha("RabbitMQ source").Must("goes ready", AllGoReady)
	// Note this is a different producer than events hub because it publishes
	// directly to RabbitMQ
	f.Setup("install producer", sourceproducer.Install(sourceproducer.WithProducerCount(eventsNumber), sourceproducer.WithCASecret()))
	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(t, eventsNumber)
			})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

//
// producer ---> rabbitmq --[source]--> recorder
//

// DirectSourceConnectionSecretTest makes sure an RabbitMQ Source with a connection secret delivers events to a sink.
func DirectSourceConnectionSecretClusterRefNS(setClusterRef bool) *feature.Feature {
	eventsNumber := 10
	f := new(feature.Feature)
	var opts []manifest.CfgFn
	if setClusterRef {
		opts = append(opts, sourcesecret.WithBrokerConfigClusterRefNS(setClusterRef))
	}

	f.Setup("install RabbitMQ source", sourcesecret.Install(opts...))
	f.Alpha("RabbitMQ source").Must("goes ready", AllGoReady)
	// Note this is a different producer than events hub because it publishes
	// directly to RabbitMQ
	f.Setup("install producer", sourceproducer.Install(sourceproducer.WithProducerCount(eventsNumber)))
	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(t, eventsNumber)
			})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

//
// producer ---> rabbitmq --[vhost(source)]--> recorder
//

// VhostSourceTest makes sure an RabbitMQ Source is created on the desired vhost.
func VHostSourceTest() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install RabbitMQ source on test-vhost", sourcevhost.Install())
	f.Alpha("RabbitMQ vhost-source ready").Must("goes ready", AllGoReady)

	// Note this is a different producer than events hub because it publishes
	// directly to RabbitMQ
	f.Setup("install producer pointing to the vhost-source", vhostsourceproducer.Install())
	f.Alpha("RabbitMQ vhost-source receiving messages").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(t, 10)
			})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

// SourceConcurrentreceiveAdapterProcessingTest verifies that the Adapter sends events
// concurrently. It does this by sending 5 events to an event recorder that
// takes 1 second to respond to each event. It waits for both events to be
// received and then calculates the time between events to determine that the
// adapter did not block on receiving a response to the first event before
// processing the second.
func SourceConcurrentReceiveAdapterProcessingTest() *feature.Feature {
	eventsNumber := 2
	f := new(feature.Feature)

	f.Setup("install test resources", source.Install())
	f.Setup("install recorder", eventshub.Install("recorder", eventshub.StartReceiver, eventshub.ResponseWaitTime(time.Second)))

	f.Requirement("recorder is addressable", k8s.IsAddressable(serviceGVR, "recorder"))
	f.Requirement("RabbitMQ broker goes ready", AllGoReady)

	f.Setup("install producer", sourceproducer.Install(sourceproducer.WithProducerCount(eventsNumber)))
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
