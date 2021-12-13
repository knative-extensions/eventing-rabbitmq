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

package broker

import (
	"os"
	"testing"

	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit/fake"
	_ "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client/fake"
	_ "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/binding/fake"
	_ "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/exchange/fake"
	_ "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/queue/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/conditions/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/service/fake"
	_ "knative.dev/pkg/injection/clients/dynamicclient/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	os.Setenv("BROKER_INGRESS_IMAGE", "ingressimage")
	os.Setenv("BROKER_INGRESS_SERVICE_ACCOUNT", "ingresssa")
	os.Setenv("BROKER_DLQ_DISPATCHER_IMAGE", "dlqdispatcherimage")
	c := NewController(ctx, &configmap.ManualWatcher{})

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}
