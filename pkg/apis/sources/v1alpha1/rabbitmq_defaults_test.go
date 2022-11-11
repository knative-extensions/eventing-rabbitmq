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

package v1alpha1

import (
	"context"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
)

const (
	sourceName     = "test-source"
	clusterRefName = "test-cluster-ref"
	sourceNS       = "test-source-namespace"
	clusterRefNS   = "test-cluster-ref-namespace"
)

func TestSourceDefaults(t *testing.T) {
	tests := map[string]struct {
		clusterRefNamespace string
		wantConfig          *RabbitmqSource
	}{
		"nil clusterRef namespace": {
			wantConfig: &RabbitmqSource{
				ObjectMeta: v1.ObjectMeta{Name: sourceName, Namespace: sourceNS},
				Spec: RabbitmqSourceSpec{
					RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{Name: clusterRefName, Namespace: sourceNS}}},
		},
		"clusterRef namespace set": {
			clusterRefNamespace: clusterRefNS,
			wantConfig: &RabbitmqSource{
				ObjectMeta: v1.ObjectMeta{Name: sourceName, Namespace: sourceNS},
				Spec: RabbitmqSourceSpec{
					RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{Name: clusterRefName, Namespace: clusterRefNS}}},
		},
	}
	for n, test := range tests {
		t.Run(n, func(t *testing.T) {
			test := test
			t.Parallel()
			gotConfig := &RabbitmqSource{
				ObjectMeta: v1.ObjectMeta{Name: sourceName, Namespace: sourceNS},
				Spec: RabbitmqSourceSpec{
					RabbitmqClusterReference: &v1beta1.RabbitmqClusterReference{Name: clusterRefName, Namespace: test.clusterRefNamespace}}}

			gotConfig.SetDefaults(context.Background())
			if gotConfig.Namespace != test.wantConfig.Namespace {
				t.Errorf("RabbitMQSource ClusterReference Namespace error want: %s,\ngot: %s) =", test.wantConfig.Namespace, gotConfig.Namespace)
			}
		})
	}
}
