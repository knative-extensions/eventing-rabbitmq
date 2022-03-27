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

package rabbit

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	rabbitduck "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	fakerabbitclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client/fake"
	"knative.dev/pkg/injection"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
)

func Test_NewService(t *testing.T) {
	ctx := context.TODO()
	ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})
	ctx, _ = fakedynamicclient.With(ctx, runtime.NewScheme())
	ctx = rabbitduck.WithDuck(ctx)

	i := fakerabbitclient.Get(ctx)
	want := &Rabbit{Interface: i}
	got := New(ctx)
	if want.Interface != got.Interface {
		t.Errorf("New function did not return a valid Rabbit interface %s %s", want, got)
	}
}

func Test_isReadyFunc(t *testing.T) {
	for _, tt := range []struct {
		name       string
		conditions []v1beta1.Condition
		want       bool
	}{{
		name: "Nil condition set",
		want: false,
	}, {
		name:       "Empty condition set",
		conditions: []v1beta1.Condition{},
		want:       false,
	}, {
		name:       "Valid condition set",
		conditions: []v1beta1.Condition{{Status: v1.ConditionTrue}, {Status: v1.ConditionTrue}},
		want:       true,
	}, {
		name:       "Invalid condition set",
		conditions: []v1beta1.Condition{{Status: v1.ConditionTrue}, {Status: v1.ConditionFalse}},
		want:       false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			got := isReady(tt.conditions)
			if got != tt.want {
				t.Errorf("unexpected error checking conditions want: %v, got: %v", tt.want, got)
			}
		})
	}
}
