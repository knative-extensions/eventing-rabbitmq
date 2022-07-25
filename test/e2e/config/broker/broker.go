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

package broker

import (
	"context"
	"embed"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed "*.yaml"
var yamls embed.FS

func Install(brokerNamespace string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		args := map[string]interface{}{
			"broker_namespace": brokerNamespace,
		}
		if _, err := manifest.InstallYamlFS(ctx, yamls, args); err != nil {
			t.Fatal(err)
		}
	}
}

func Uninstall(brokerNamespace string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		args := map[string]interface{}{
			"broker_namespace": brokerNamespace,
		}
		if _, err := manifest.InstallYamlFS(ctx, yamls, args); err != nil {
			t.Fatal(err)
		}

		kubeClient := kubeclient.Get(ctx)

		if err := kubeClient.CoreV1().Namespaces().Delete(context.Background(), brokerNamespace, metav1.DeleteOptions{}); err != nil {
			t.Fatal(err)
		}
	}
}
