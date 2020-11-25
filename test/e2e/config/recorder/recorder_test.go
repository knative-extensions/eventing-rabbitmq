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

package recorder_test

import (
	"os"

	"knative.dev/reconciler-test/pkg/manifest"
)

func Example() {
	images := map[string]string{
		"ko://knative.dev/eventing-rabbitmq/test/e2e/cmd/recorder": "valid://container/image/recorder",
	}
	cfg := map[string]interface{}{
		"namespace": "bar",
	}

	files, err := manifest.ExecuteLocalYAML(images, cfg)
	if err != nil {
		panic(err)
	}

	manifest.OutputYAML(os.Stdout, files)
	// Output:
	// apiVersion: v1
	// kind: ServiceAccount
	// metadata:
	//   name: recorder
	//   namespace: bar
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRole
	// metadata:
	//   name: event-watcher-bar
	// rules:
	//   - apiGroups: [ "" ]
	//     resources:
	//       - "namespaces"
	//     verbs:
	//       - "get"
	//       - "list"
	//       - "watch"
	//   - apiGroups: [ "" ]
	//     resources:
	//       - "events"
	//     verbs:
	//       - "get"
	//       - "list"
	//       - "create"
	//       - "update"
	//       - "delete"
	//       - "patch"
	//       - "watch"
	// ---
	// apiVersion: rbac.authorization.k8s.io/v1
	// kind: ClusterRoleBinding
	// metadata:
	//   name: event-watcher-bar
	// roleRef:
	//   apiGroup: rbac.authorization.k8s.io
	//   kind: ClusterRole
	//   name: event-watcher-bar
	// subjects:
	//   - kind: ServiceAccount
	//     name: recorder
	//     namespace: bar
	// ---
	// apiVersion: apps/v1
	// kind: Deployment
	// metadata:
	//   name: recorder
	//   namespace: bar
	// spec:
	//   replicas: 1
	//   selector:
	//     matchLabels: &labels
	//       app: recorder
	//   template:
	//     metadata:
	//       labels: *labels
	//     spec:
	//       serviceAccount: recorder
	//       containers:
	//         - name: recorder
	//           image: valid://container/image/recorder
	//           env:
	//             - name: SYSTEM_NAMESPACE
	//               value: bar
	//             - name: OBSERVER_NAME
	//               value: recorder-bar
	//             - name: K8S_EVENT_SINK
	//               value: '{"apiVersion": "v1", "kind": "Namespace", "name": "bar"}'
	// ---
	// kind: Service
	// apiVersion: v1
	// metadata:
	//   name: recorder
	//   namespace: bar
	// spec:
	//   selector:
	//     app: recorder
	//   ports:
	//     - protocol: TCP
	//       port: 80
	//       targetPort: 8080
}
