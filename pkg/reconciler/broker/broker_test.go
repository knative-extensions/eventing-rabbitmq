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
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	clientgotesting "k8s.io/client-go/testing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/eventing/pkg/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1addr "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	v1a1addr "knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	v1b1addr "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"

	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	rt "knative.dev/eventing/pkg/reconciler/testing/v1"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	finalizerName = "brokers.eventing.knative.dev"
	brokerClass   = "RabbitMQBroker"
	//	systemNS      = "knative-testing"
	testNS     = "test-namespace"
	brokerName = "test-broker"

	rabbitSecretName = "test-secret"
	rabbitSecretData = "amqp://testrabbit"
	// Note that this seems to fail differently on CI vs. locally.
	// on Github actions we see this:
	dnsFailureMsg = `Failed to create exchange: dial tcp: lookup testrabbit: Temporary failure in name resolution`
	// on macbook I see this:
	//dnsFailureMsg = `Failed to create exchange: dial tcp: lookup testrabbit: no such host`

	// Ditto here...
	dnsFailureEvent = `dial tcp: lookup testrabbit: Temporary failure in name resolution`
	//dnsFailureEvent = `dial tcp: lookup testrabbit: no such host`
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, brokerName)

	/* TODO: Use once we can fake the rabbit stuff
	ingressServiceName = "broker-ingress"
	brokerAddress = &apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, systemNS, utils.GetClusterDomainName()),
		Path:   fmt.Sprintf("/%s/%s", testNS, brokerName),
	}
	*/
)

func init() {
	// Add types to scheme
	_ = eventingv1.AddToScheme(scheme.Scheme)
	_ = duckv1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "Broker not found",
			Key:  testKey,
		}, {
			Name: "Broker is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerDeletionTimestamp),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `secrets "test-secret" not found`),
			},
			WantErr: true,
		}, {
			Name: "nil config",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `Broker.Spec.Config is required`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithExchangeFailed("ExchangeCredentialsUnavailable", "Failed to get arguments for creating exchange: Broker.Spec.Config is required")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "nil config, missing name",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(&duckv1.KReference{Kind: "Secret", APIVersion: "v1"}),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-broker": missing field(s): spec.config.name`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(&duckv1.KReference{Kind: "Secret", APIVersion: "v1"}),
					WithInitBrokerConditions,
					WithExchangeFailed("ExchangeCredentialsUnavailable", "Failed to get arguments for creating exchange: broker.spec.config.[name, namespace] are required")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "Secret not found",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeFailed("ExchangeCredentialsUnavailable", `Failed to get arguments for creating exchange: secrets "test-secret" not found`)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `secrets "test-secret" not found`),
			},
			WantErr: true,
		}, {
			Name: "Exchange create fails - malformed uri",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret("invalid data"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeFailed("ExchangeFailure", `Failed to create exchange: AMQP scheme must be either 'amqp://' or 'amqps://'`)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `AMQP scheme must be either 'amqp://' or 'amqps://'`),
			},
			WantErr: true,
		}, {
			Name: "Exchange create fails - can not talk to rabbit",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitSecretData),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeFailed("ExchangeFailure", dnsFailureMsg)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", dnsFailureEvent),
			},
			WantErr: true,
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, rt.MakeFactory(func(ctx context.Context, listers *rt.Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = conditions.WithDuck(ctx)
		eventingv1.RegisterAlternateBrokerConditionSet(rabbitBrokerCondSet)
		r := &Reconciler{
			eventingClientSet:  fakeeventingclient.Get(ctx),
			dynamicClientSet:   fakedynamicclient.Get(ctx),
			kubeClientSet:      fakekubeclient.Get(ctx),
			endpointsLister:    listers.GetEndpointsLister(),
			kresourceTracker:   duck.NewListableTracker(ctx, conditions.Get, func(types.NamespacedName) {}, 0),
			addressableTracker: duck.NewListableTracker(ctx, v1a1addr.Get, func(types.NamespacedName) {}, 0),
			uriResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			brokerClass:        "RabbitMQBroker",
		}
		return broker.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetBrokerLister(),
			controller.GetEventRecorder(ctx),
			r, "RabbitMQBroker")

	},
		false,
		logger,
	))
}

func createSecret(data string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      rabbitSecretName,
		},
		Data: map[string][]byte{
			"brokerURL": []byte(data),
		},
	}
}

func config() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitSecretName,
		Namespace:  testNS,
		Kind:       "Secret",
		APIVersion: "v1",
	}
}

// FilterLabels generates the labels present on all resources representing the filter of the given
// Broker.
/* TODO: Enable
func IngressLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerRole": "ingress",
	}
}
*/

func patchFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

/*
// TODO Enable these tests
func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}
*/
