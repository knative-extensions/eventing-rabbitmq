/*
Copyright 2022 The Knative Authors

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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeClient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

func CleanupConnectionSecret() *feature.Feature {
	f := new(feature.Feature)
	f.Teardown("clean up topology operator volumes and mounts", CleanupSecret)
	return f
}

func CleanupSecret(ctx context.Context, t feature.T) {
	namespace := environment.FromContext(ctx).Namespace()
	_ = kubeClient.Get(ctx).CoreV1().Secrets(namespace).Delete(ctx, "rabbitmqc-default-user", metav1.DeleteOptions{})
}
