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

package brokertrigger

import (
	"context"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

func init() {
	environment.RegisterPackage(manifest.ImagesLocalYaml()...)
}

type Topology struct {
	MessageCount, PrefetchCount int
	Triggers                    []duckv1.KReference
}

func Install(topology Topology) feature.StepFn {
	if topology.PrefetchCount == 0 {
		topology.PrefetchCount = 1
	}

	args := map[string]interface{}{
		"messageCount":  topology.MessageCount,
		"triggers":      topology.Triggers,
		"prefetchCount": topology.PrefetchCount,
	}
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallLocalYaml(ctx, args); err != nil {
			t.Fatal(err)
		}
	}
}
