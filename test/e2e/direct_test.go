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

package e2e

import (
	"context"

	"knative.dev/eventing-rabbitmq/test/e2e/config/brokertrigger"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	_ "knative.dev/pkg/system/testing"
)

//
// producer ---> broker --[trigger]--> recorder
//

// DirectTestBrokerImpl makes sure an RabbitMQ Broker delivers events to a single consumer.
func DirectTestBroker() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install test resources", brokertrigger.Install(brokertrigger.Topology{
		MessageCount: 5,
		Triggers: []duckv1.KReference{
			{
				Kind: "Service",
				Name: "recorder",
			},
		},
	}))

	f.Alpha("RabbitMQ broker").Must("goes ready", AllGoReady)
	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(t, 5)
			})
	return f
}
