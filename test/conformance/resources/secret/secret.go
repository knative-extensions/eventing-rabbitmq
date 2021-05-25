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

package secret

import (
	"context"
	"fmt"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/reconciler-test/pkg/environment"

	r "knative.dev/eventing-rabbitmq/test/conformance/resources/rabbitmqcluster"
)

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "rabbitmq.com", Version: "v1beta1", Resource: "rabbitmqclusters"}
}

// Install will create a RabbitmqCluster resource, augmented with the config fn options.
func Install(name, rabbitmqCluster string, ctx context.Context, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	env := environment.FromContext(ctx)
	u, err := r.RabbitmqURLFromRabbit(ctx, env.Namespace(), rabbitmqCluster)
	if err != nil {
		panic(fmt.Sprintf("Failed to get rabbit secret: %+v", err))
	}
	cfg["brokerURL"] = u.String()
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallLocalYaml(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}
