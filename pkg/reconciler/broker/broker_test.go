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
	"net/http"
	"net/http/httptest"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
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

	"github.com/NeowayLabs/wabbit/amqptest/server"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
)

const (
	brokerClass = "RabbitMQBroker"
	testNS      = "test-namespace"
	brokerName  = "test-broker"

	rabbitSecretName       = "test-secret"
	rabbitBrokerSecretName = "test-broker-broker-rabbit"
	rabbitURL              = "amqp://localhost:5672/%2f"
	ingressImage           = "ingressimage"

	deadLetterSinkKind       = "Service"
	deadLetterSinkName       = "deadLetterSink-name"
	deadLetterSinkAPIVersion = "serving.knative.dev/v1"

	dispatcherImage = "dispatcherimage"

	bindingList = `
[
  {
    "source": "knative-test-broker",
    "vhost": "/",
    "destination": "test-namespace-test-broker",
    "destination_type": "queue",
    "routing_key": "test-namespace-test-broker",
    "arguments": {
      "x-match":  "all",
      "x-knative-trigger": "test-trigger",
      "type": "dev.knative.sources.ping"
    },
    "properties_key": "test-namespace-test-broker"
  }
]
`
)

var (
	TrueValue = true

	testKey = fmt.Sprintf("%s/%s", testNS, brokerName)

	ingressServiceName = "test-broker-broker-ingress"
	brokerAddress      = &apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName()),
	}
	deadLetterSinkAddress = &apis.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/subscriber/",
	}

	five         = int32(5)
	policy       = eventingduckv1.BackoffPolicyExponential
	backoffDelay = "PT30S"
	delivery     = &eventingduckv1.DeliverySpec{
		DeadLetterSink: &duckv1.Destination{
			URI: deadLetterSinkAddress,
		},
		Retry:         &five,
		BackoffPolicy: &policy,
		BackoffDelay:  &backoffDelay,
	}
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
			Name: "Broker deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				brokerWithFinalizer(),
				createSecret(rabbitURL),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchRemoveFinalizers(testNS, brokerName),
			},
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
					WithExchangeFailed("ExchangeFailure", `Failed to create exchange: Network unreachable`)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `Network unreachable`),
			},
			WantErr: true,
		}, {
			Name: "Secret create fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "secrets"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeReady(),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretFailed("SecretFailure", `Failed to reconcile secret: inducing failure for create secrets`)),
			}},
			WantCreates: []runtime.Object{
				createExchangeSecret(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create secrets`),
			},
			WantErr: true,
		}, {
			Name: "Secret update fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
				createDifferentExchangeSecret(),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "secrets"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeReady(),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretFailed("SecretFailure", `Failed to reconcile secret: inducing failure for update secrets`)),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: createExchangeSecret(),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for update secrets`),
			},
			WantErr: true,
		}, {
			Name: "Deployment create fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
				createExchangeSecret(),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeReady(),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretReady(),
					WithIngressFailed("DeploymentFailure", `Failed to reconcile deployment: inducing failure for create deployments`)),
			}},
			WantCreates: []runtime.Object{
				createIngressDeployment(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create deployments`),
			},
			WantErr: true,
		}, {
			Name: "Deployment update fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
				createExchangeSecret(),
				createDifferentIngressDeployment(),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeReady(),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretReady(),
					WithIngressFailed("DeploymentFailure", `Failed to reconcile deployment: inducing failure for update deployments`)),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: createIngressDeployment(),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for update deployments`),
			},
			WantErr: true,
		}, {
			Name: "Service create fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
				createExchangeSecret(),
				createIngressDeployment(),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeReady(),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretReady(),
					WithIngressFailed("ServiceFailure", `Failed to reconcile service: inducing failure for create services`)),
			}},
			WantCreates: []runtime.Object{
				createIngressService(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create services`),
			},
			WantErr: true,
		}, {
			Name: "Service update fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
				createExchangeSecret(),
				createIngressDeployment(),
				createDifferentIngressService(),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithExchangeReady(),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretReady(),
					WithIngressFailed("ServiceFailure", `Failed to reconcile service: inducing failure for update services`)),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: createIngressService(),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for update services`),
			},
			WantErr: true,
		}, {
			Name: "Exchange created - endpoints not ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithIngressFailed("ServiceFailure", `Failed to reconcile service: endpoints "test-broker-broker-ingress" not found`),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretReady(),
					WithExchangeReady()),
			}},
			WantCreates: []runtime.Object{
				createExchangeSecret(),
				createIngressDeployment(),
				createIngressService(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "test-broker-broker-ingress" not found`),
			},
			WantErr: true,
		}, {
			Name: "Exchange created - endpoints ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
				rt.NewEndpoints(ingressServiceName, testNS,
					rt.WithEndpointsLabels(IngressLabels()),
					rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithIngressAvailable(),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretReady(),
					WithBrokerAddressURI(brokerAddress),
					WithExchangeReady()),
			}},
			WantCreates: []runtime.Object{
				createExchangeSecret(),
				createIngressDeployment(),
				createIngressService(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
			},
			WantErr: false,
		}, {
			Name: "Exchange created with DLQ dispatcher created",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config()),
					WithBrokerDelivery(delivery),
					WithInitBrokerConditions),
				createSecret(rabbitURL),
				rt.NewEndpoints(ingressServiceName, testNS,
					rt.WithEndpointsLabels(IngressLabels()),
					rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithBrokerDelivery(delivery),
					WithIngressAvailable(),
					WithDLXReady(),
					WithDeadLetterSinkReady(),
					WithSecretReady(),
					WithBrokerAddressURI(brokerAddress),
					WithExchangeReady()),
			}},
			WantCreates: []runtime.Object{
				createExchangeSecret(),
				createIngressDeployment(),
				createIngressService(),
				createDispatcherDeployment(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`),
			},
			WantErr: false,
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, rt.MakeFactory(func(ctx context.Context, listers *rt.Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = conditions.WithDuck(ctx)
		eventingv1.RegisterAlternateBrokerConditionSet(rabbitBrokerCondSet)
		fakeServer := server.NewServer(rabbitURL)
		fakeServer.Start()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, bindingList)
		}))
		r := &Reconciler{
			eventingClientSet:  fakeeventingclient.Get(ctx),
			dynamicClientSet:   fakedynamicclient.Get(ctx),
			kubeClientSet:      fakekubeclient.Get(ctx),
			endpointsLister:    listers.GetEndpointsLister(),
			serviceLister:      listers.GetServiceLister(),
			secretLister:       listers.GetSecretLister(),
			deploymentLister:   listers.GetDeploymentLister(),
			kresourceTracker:   duck.NewListableTracker(ctx, conditions.Get, func(types.NamespacedName) {}, 0),
			addressableTracker: duck.NewListableTracker(ctx, v1a1addr.Get, func(types.NamespacedName) {}, 0),
			uriResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			brokerClass:        "RabbitMQBroker",
			dialerFunc:         dialer.TestDialer,
			adminURL:           ts.URL,
			ingressImage:       ingressImage,
			dispatcherImage:    dispatcherImage,
		}
		return broker.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetBrokerLister(),
			controller.GetEventRecorder(ctx),
			r, "RabbitMQBroker",
			controller.Options{FinalizerName: finalizerName},
		)

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

// This is the secret that Broker creates for each broker.
func createExchangeSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      rabbitBrokerSecretName,
			Labels:    map[string]string{"eventing.knative.dev/broker": "test-broker"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
		},
		StringData: map[string]string{
			"brokerURL": rabbitURL,
		},
	}
}

func createDifferentExchangeSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      rabbitBrokerSecretName,
			Labels:    map[string]string{"eventing.knative.dev/broker": "test-broker"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
		},
		StringData: map[string]string{
			"brokerURL": "different stuff",
		},
	}
}

func createIngressDeployment() *appsv1.Deployment {
	args := &resources.IngressArgs{
		Broker:             &eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: testNS}},
		Image:              ingressImage,
		RabbitMQSecretName: rabbitBrokerSecretName,
		BrokerUrlSecretKey: resources.BrokerURLSecretKey,
	}
	return resources.MakeIngressDeployment(args)
}

func createDifferentIngressDeployment() *appsv1.Deployment {
	args := &resources.IngressArgs{
		Broker:             &eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: testNS}},
		Image:              "differentImage",
		RabbitMQSecretName: rabbitBrokerSecretName,
		BrokerUrlSecretKey: resources.BrokerURLSecretKey,
	}
	return resources.MakeIngressDeployment(args)
}

func createIngressService() *corev1.Service {
	return resources.MakeIngressService(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: testNS}})
}

func createDifferentIngressService() *corev1.Service {
	svc := resources.MakeIngressService(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: testNS}})
	svc.Spec.Ports = []corev1.ServicePort{{Name: "diff", Port: 9999}}
	return svc
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
func IngressLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerRole": "ingress",
	}
}

func brokerWithFinalizer() *eventingv1.Broker {
	b := NewBroker(brokerName, testNS,
		WithBrokerClass(brokerClass),
		WithBrokerConfig(config()),
		WithInitBrokerConditions,
		WithBrokerDeletionTimestamp)
	b.Finalizers = []string{finalizerName}
	return b
}

func patchFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func createDispatcherDeployment() *appsv1.Deployment {
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerName,
			Namespace: testNS,
		},
		Spec: eventingv1.BrokerSpec{
			Config:   config(),
			Delivery: delivery,
		},
	}
	args := &resources.DispatcherArgs{
		Broker:             broker,
		Image:              dispatcherImage,
		RabbitMQSecretName: rabbitBrokerSecretName,
		QueueName:          "test-namespace-test-broker-dlq",
		BrokerUrlSecretKey: "brokerURL",
		BrokerIngressURL:   brokerAddress,
		Subscriber:         deadLetterSinkAddress,
	}
	return resources.MakeDispatcherDeployment(args)
}
