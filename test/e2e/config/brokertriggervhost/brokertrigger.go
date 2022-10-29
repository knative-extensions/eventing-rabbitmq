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

package brokertriggervhost

import (
	"context"
	"embed"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed "*.yaml"
var yamls embed.FS

type Topology struct {
	Parallelism int
	Triggers    []duckv1.KReference
}

func Install(topology Topology) feature.StepFn {
	if topology.Parallelism == 0 {
		topology.Parallelism = 1
	}

	args := map[string]interface{}{
		"triggers":    topology.Triggers,
		"Parallelism": topology.Parallelism,
	}
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, yamls, args); err != nil {
			t.Fatal(err)
		}
	}
}
