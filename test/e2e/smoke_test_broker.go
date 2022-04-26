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
	"knative.dev/eventing-rabbitmq/test/e2e/config/smoke/broker"
	"knative.dev/reconciler-test/pkg/feature"
)

// SmokeTestBrokerImpl makes sure an RabbitMQ Broker goes ready.
func SmokeTestBroker() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install a broker", broker.Install())
	f.Alpha("RabbitMQ broker").Must("goes ready", AllGoReady)
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}
