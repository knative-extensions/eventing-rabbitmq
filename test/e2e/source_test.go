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

package e2e

import (
	"context"

	"knative.dev/eventing-rabbitmq/test/e2e/config/source"
	"knative.dev/eventing-rabbitmq/test/e2e/config/sourceproducer"
	"knative.dev/eventing-rabbitmq/test/e2e/config/sourcevhost"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "knative.dev/pkg/system/testing"
)

//
// producer ---> rabbitmq --[source]--> recorder
//

// DirectSourceTest makes sure an RabbitMQ Source delivers events to a sink.
func DirectSourceTest() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install RabbitMQ source", source.Install())
	f.Alpha("RabbitMQ source").Must("goes ready", AllGoReady)
	// Note this is a different producer than events hub because it publishes
	// directly to RabbitMQ
	f.Setup("install producer", sourceproducer.Install())
	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(t, 5)
			})

	return f
}

// VhostSourceTest makes sure an RabbitMQ Source is created on the desired vhost.
func VHostSourceTest() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install RabbitMQ source on test-vhost", sourcevhost.Install())
	f.Alpha("RabbitMQ source").Must("goes ready", AllGoReady)
	// TODO: still needs some more logic to test all of it
	return f
}
