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

package direct

import (
	"context"
	"testing"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

func init() {
	environment.RegisterPackage(manifest.ImagesLocalYaml()...)
}

func Install() feature.StepFn {
	return func(ctx context.Context, t *testing.T) {
		if _, err := manifest.InstallLocalYaml(ctx, map[string]interface{}{"producerCount": 5}); err != nil {
			t.Fatal(err)
		}
	}
}
