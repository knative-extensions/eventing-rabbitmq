//go:build tools
// +build tools

/*
Copyright 2020 The Knative Authors.

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

package tools

import (
	_ "k8s.io/code-generator"
	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "k8s.io/code-generator/cmd/defaulter-gen"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
	_ "knative.dev/pkg/codegen/cmd/injection-gen"

	_ "knative.dev/hack"

	// codegen: hack/generate-knative.sh
	_ "knative.dev/pkg/hack"

	// Test images from eventing
	_ "knative.dev/eventing/cmd/heartbeats"
	_ "knative.dev/eventing/test/test_images/event-sender"
	_ "knative.dev/eventing/test/test_images/performance"
	_ "knative.dev/eventing/test/test_images/print"

	// Licenseclassifier
	_ "github.com/google/licenseclassifier"

	// For chaos testing the leaderelection stuff.
	_ "knative.dev/pkg/leaderelection/chaosduck"

	// eventshub is a cloudevents sender/receiver utility for e2e testing.
	_ "knative.dev/reconciler-test/cmd/eventshub"

	// For performance testing, this needs to be imported
	_ "knative.dev/pkg/test/mako/stub-sidecar"
)
