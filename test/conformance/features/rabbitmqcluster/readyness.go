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

package rabbitmqcluster

import (
	"fmt"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/eventing-rabbitmq/test/conformance/resources/rabbitmqcluster"
)

// GoesReady returns a feature that will create a RabbitmqCluster
// and confirm it becomes ready
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := new(feature.Feature)

	f.Setup(fmt.Sprintf("install rabbitmqcluster %q", name), rabbitmqcluster.Install(name, cfg...))

	f.Requirement("RabbitmqCluster is ready", rabbitmqcluster.IsReady(name))

	return f
}
