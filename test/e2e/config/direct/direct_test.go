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

package direct_test

import (
	"os"

	"knative.dev/reconciler-test/pkg/manifest"
)

func Example() {
	images := map[string]string{
		"ko://knative.dev/eventing-rabbitmq/test/e2e/cmd/producer": "valid://container/image/producer",
	}
	cfg := map[string]interface{}{
		// "name":          "foo", // TODO: pass in the broker name.
		"namespace":     "bar",
		"producerCount": "42",
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: eventing.knative.dev/v1
	// kind: Broker
	// metadata:
	//   name: testbroker
	//   namespace: bar
	//   annotations:
	//     eventing.knative.dev/broker.class: RabbitMQBroker
	// spec:
	//   config:
	//     apiVersion: rabbitmq.com/v1beta1
	//     kind: RabbitmqCluster
	//     name: rabbitmqc
	// ---
	// apiVersion: apps/v1
	// kind: Deployment
	// metadata:
	//   name: producer
	//   namespace: bar
	// spec:
	//   replicas: 1
	//   selector:
	//     matchLabels: &labels
	//       app: producer
	//   template:
	//     metadata:
	//       labels: *labels
	//     spec:
	//       containers:
	//         - name: producer
	//           image: valid://container/image/producer
	//           env:
	//             - name: COUNT
	//               value: '42'
	//             - name: K_SINK
	//               value: http://testbroker-broker-ingress.bar.svc.cluster.local
	// ---
	// apiVersion: eventing.knative.dev/v1
	// kind: Trigger
	// metadata:
	//   name: testtrigger
	//   namespace: bar
	// spec:
	//   broker: testbroker
	//   subscriber:
	//     ref:
	//       apiVersion: v1
	//       kind: Service
	//       name: recorder
}
