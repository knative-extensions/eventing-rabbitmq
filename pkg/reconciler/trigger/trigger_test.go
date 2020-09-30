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

package trigger

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"knative.dev/pkg/network"

	"github.com/NeowayLabs/wabbit/amqptest/server"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	clientgotesting "k8s.io/client-go/testing"
	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	broker "knative.dev/eventing-rabbitmq/pkg/reconciler/broker"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	"knative.dev/eventing/pkg/duck"
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
	rtv1alpha1 "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	systemNS    = "knative-testing"
	testNS      = "test-namespace"
	brokerClass = "RabbitMQBroker"
	brokerName  = "test-broker"

	rabbitSecretName = "test-broker-broker-rabbit"

	triggerName = "test-trigger"
	triggerUID  = "test-trigger-uid"

	rabbitURL = "amqp://localhost:5672/%2f"

	dispatcherImage = "dispatcherimage"

	subscriberURI = "http://example.com/subscriber/"

	pingSourceName       = "test-ping-source"
	testSchedule         = "*/2 * * * *"
	testData             = "data"
	sinkName             = "testsink"
	dependencyAnnotation = "{\"kind\":\"PingSource\",\"name\":\"test-ping-source\",\"apiVersion\":\"sources.knative.dev/v1beta1\"}"
	currentGeneration    = 1
	outdatedGeneration   = 0

	subscriberKind    = "Service"
	subscriberName    = "subscriber-name"
	subscriberGroup   = "serving.knative.dev"
	subscriberVersion = "v1"

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
	testKey = fmt.Sprintf("%s/%s", testNS, triggerName)

	subscriberAPIVersion = fmt.Sprintf("%s/%s", subscriberGroup, subscriberVersion)

	subscriberGVK = metav1.GroupVersionKind{
		Group:   subscriberGroup,
		Version: subscriberVersion,
		Kind:    subscriberKind,
	}

	ingressServiceName = "broker-ingress"

	brokerDestv1 = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1",
		},
	}
	sinkDNS = "sink.mynamespace.svc." + network.GetClusterDomainName()
	sinkURI = "http://" + sinkDNS

	brokerAddress = &apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, systemNS, network.GetClusterDomainName()),
	}
	subscriberAddress = &apis.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/subscriber/",
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
			Name: "Trigger not found",
			Key:  testKey,
		}, {
			Name:    "Trigger is being deleted",
			Key:     testKey,
			Objects: []runtime.Object{NewTrigger(triggerName, testNS, brokerName, WithTriggerDeleted)},
		}, {
			Name: "Trigger being deleted, not my broker",
			Key:  testKey,
			Objects: []runtime.Object{
				broker.NewBroker(brokerName, testNS,
					broker.WithBrokerClass("not-my-broker"),
					broker.WithBrokerConfig(config()),
					broker.WithInitBrokerConditions),
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerDeleted,
					WithTriggerSubscriberURI(subscriberURI)),
			},
		}, {
			Name: "Trigger delete fails - with finalizer - no secret",
			Key:  testKey,
			Objects: []runtime.Object{
				broker.NewBroker(brokerName, testNS,
					broker.WithBrokerClass(brokerClass),
					broker.WithBrokerConfig(config()),
					broker.WithInitBrokerConditions),
				triggerWithFinalizerReady(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `secrets "test-broker-broker-rabbit" not found`),
			},
			WantErr: true,
		}, {
			Name: "Trigger deleted - with finalizer and secret",
			Key:  testKey,
			Objects: []runtime.Object{
				broker.NewBroker(brokerName, testNS,
					broker.WithBrokerClass(brokerClass),
					broker.WithBrokerConfig(config()),
					broker.WithInitBrokerConditions),
				createSecret(rabbitURL),
				triggerWithFinalizerReady(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchRemoveFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
		}, {
			Name: "Broker does not exist",
			Key:  testKey,
			Objects: []runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "test-broker" does not exist`)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
		}, {
			Name: "Not my broker class - no status updates",
			Key:  testKey,
			Objects: []runtime.Object{
				broker.NewBroker(brokerName, testNS,
					broker.WithBrokerClass("not-my-broker"),
					broker.WithBrokerConfig(config()),
					broker.WithInitBrokerConditions),
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI)),
			},
			// BUG. We should not be setting finalizers on objects that we do not own.
			// https://github.com/knative/pkg/issues/1734
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
		}, {
			Name: "Broker not reconciled yet",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerClass),
					WithBrokerConfig(config())),
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerNotConfigured()),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
		}, {
			Name: "Broker not ready yet",
			Key:  testKey,
			Objects: []runtime.Object{
				broker.NewBroker(brokerName, testNS,
					broker.WithBrokerClass(brokerClass),
					broker.WithBrokerConfig(config()),
					broker.WithInitBrokerConditions,
					broker.WithExchangeFailed("noexchange", "NoExchange")),
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerFailed("noexchange", "NoExchange")),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
		}, {
			Name: "Creates everything ok",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				triggerWithFilter(),
				createSecret(rabbitURL),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: triggerWithFilterReady(),
			}},
		}, {
			Name: "Creates everything with ref",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS)),
				createSecret(rabbitURL),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscribed(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			}},
		}, {
			Name: "Fails to resolve ref",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				makeSubscriberNotAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS)),
				createSecret(rabbitURL),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", `address not set for &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}`),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedFailed("Unable to get the Subscriber's URI", `address not set for &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}`)),
			}},
		}, {
			Name: "Invalid secret - missing key",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				createInvalidSecret(rabbitURL),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", "Secret missing key brokerURL"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerDependencyFailed("SecretFailure", "Secret missing key brokerURL")),
			}},
		}, {
			Name: "Deployment creation fails",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				createSecret(rabbitURL),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create deployments"),
			},
			WantErr: true,
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerDependencyFailed("DeploymentFailure", "inducing failure for create deployments")),
			}},
		}, {
			Name: "Deployment creation fails",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				createSecret(rabbitURL),
				createDifferentDispatcherDeployment(),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update deployments"),
			},
			WantErr: true,
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: createDispatcherDeployment(),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerDependencyFailed("DeploymentFailure", "inducing failure for update deployments")),
			}},
		}, {
			Name: "Fail binding",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				createSecret(rabbitURL),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscribed(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			}},
		}, {
			Name: "Dependency doesn't exist",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
				Eventf(corev1.EventTypeWarning, "InternalError", "propagating dependency readiness: getting the dependency: pingsources.sources.knative.dev \"test-ping-source\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerDependencyFailed("DependencyDoesNotExist", "Dependency does not exist: pingsources.sources.knative.dev \"test-ping-source\" not found"),
				),
			}},
			WantErr: true,
		}, {
			Name: "The status of Dependency is False",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				makeFalseStatusPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				),
				createSecret(rabbitURL),
			},
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscribed(),
					WithTriggerSubscriberResolvedSucceeded(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI),
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerDependencyFailed("NotFound", ""),
				),
			}},
		}, {
			Name: "The status of Dependency is Unknown",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				makeUnknownStatusCronJobSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				),
				createSecret(rabbitURL),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyUnknown("", ""),
				),
			}},
		},
		{
			Name: "Dependency generation not equal",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				makeGenerationNotEqualPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				),
				createSecret(rabbitURL),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyUnknown("GenerationNotEqual", fmt.Sprintf("The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", currentGeneration, outdatedGeneration))),
			}},
		},
		{
			Name: "Dependency ready",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				makeReadyPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				),
				createSecret(rabbitURL),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = conditions.WithDuck(ctx)
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
			brokerLister:       listers.GetBrokerLister(),
			triggerLister:      listers.GetTriggerLister(),
			deploymentLister:   listers.GetDeploymentLister(),
			kresourceTracker:   duck.NewListableTracker(ctx, conditions.Get, func(types.NamespacedName) {}, 0),
			addressableTracker: duck.NewListableTracker(ctx, v1addr.Get, func(types.NamespacedName) {}, 0),
			uriResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			brokerClass:        "RabbitMQBroker",
			dialerFunc:         dialer.TestDialer,
			transport:          ts.Client().Transport,
			adminURL:           ts.URL,
			dispatcherImage:    dispatcherImage,
		}
		return trigger.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetTriggerLister(),
			controller.GetEventRecorder(ctx),
			r,
			controller.Options{FinalizerName: finalizerName},
		)

	},
		false,
		logger,
	))
}

func config() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitSecretName,
		Namespace:  testNS,
		Kind:       "Secret",
		APIVersion: "v1",
	}
}

func makeFalseStatusPingSource() *sourcesv1beta1.PingSource {
	return rtv1alpha1.NewPingSourceV1Beta1(pingSourceName, testNS, rtv1alpha1.WithPingSourceV1B1SinkNotFound)
}

func makeUnknownStatusCronJobSource() *sourcesv1beta1.PingSource {
	cjs := rtv1alpha1.NewPingSourceV1Beta1(pingSourceName, testNS)
	cjs.Status.InitializeConditions()
	return cjs
}

func makeGenerationNotEqualPingSource() *sourcesv1beta1.PingSource {
	c := makeFalseStatusPingSource()
	c.Generation = currentGeneration
	c.Status.ObservedGeneration = outdatedGeneration
	return c
}

func makeReadyPingSource() *sourcesv1beta1.PingSource {
	u, _ := apis.ParseURL(sinkURI)
	return rtv1alpha1.NewPingSourceV1Beta1(pingSourceName, testNS,
		rtv1alpha1.WithPingSourceV1B1Spec(sourcesv1beta1.PingSourceSpec{
			Schedule: testSchedule,
			JsonData: testData,
			SourceSpec: duckv1.SourceSpec{
				Sink: brokerDestv1,
			},
		}),
		rtv1alpha1.WithInitPingSourceV1B1Conditions,
		rtv1alpha1.WithPingSourceV1B1Deployed,
		rtv1alpha1.WithPingSourceV1B1CloudEventAttributes,
		rtv1alpha1.WithPingSourceV1B1Sink(u),
	)
}

// Create Ready Broker with proper annotations.
func ReadyBroker() *eventingv1.Broker {
	return broker.NewBroker(brokerName, testNS,
		broker.WithBrokerClass(brokerClass),
		broker.WithInitBrokerConditions,
		broker.WithBrokerConfig(config()),
		broker.WithIngressAvailable(),
		broker.WithSecretReady(),
		broker.WithBrokerAddressURI(brokerAddress),
		broker.WithExchangeReady())
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

func triggerWithFilter() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithTriggerSubscriberURI(subscriberURI))
	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: eventingv1.TriggerFilterAttributes(map[string]string{"type": "dev.knative.sources.ping"}),
	}
	return t
}

func triggerWithFilterReady() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDependencyReady(),
		WithTriggerSubscribed(),
		WithTriggerSubscriberResolvedSucceeded(),
		WithTriggerStatusSubscriberURI(subscriberURI))
	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: eventingv1.TriggerFilterAttributes(map[string]string{"type": "dev.knative.sources.ping"}),
	}
	return t
}

func triggerWithFinalizerReady() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDeleted,
		WithTriggerDependencyReady(),
		WithTriggerSubscribed(),
		WithTriggerSubscriberResolvedSucceeded(),
		WithTriggerStatusSubscriberURI(subscriberURI))
	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: eventingv1.TriggerFilterAttributes(map[string]string{"type": "dev.knative.sources.ping"}),
	}
	t.Finalizers = []string{finalizerName}
	return t
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

func createInvalidSecret(data string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      rabbitSecretName,
		},
		Data: map[string][]byte{
			"randokey": []byte(data),
		},
	}
}

func createDispatcherDeployment() *appsv1.Deployment {
	args := &resources.DispatcherArgs{
		Trigger: &eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: testNS,
				UID:       triggerUID,
			},
			Spec: eventingv1.TriggerSpec{
				Broker: brokerName,
			},
		},
		Image:              dispatcherImage,
		RabbitMQSecretName: rabbitSecretName,
		QueueName:          triggerName,
		BrokerUrlSecretKey: "brokerURL",
		BrokerIngressURL:   brokerAddress,
		Subscriber:         subscriberAddress,
	}
	return resources.MakeDispatcherDeployment(args)
}

func createDifferentDispatcherDeployment() *appsv1.Deployment {
	args := &resources.DispatcherArgs{
		Trigger: &eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: testNS,
				UID:       triggerUID,
			},
			Spec: eventingv1.TriggerSpec{
				Broker: brokerName,
			},
		},
		Image:              "differentdispatcherimage",
		RabbitMQSecretName: rabbitSecretName,
		QueueName:          triggerName,
		BrokerUrlSecretKey: "brokerURL",
		BrokerIngressURL:   brokerAddress,
		Subscriber:         subscriberAddress,
	}
	return resources.MakeDispatcherDeployment(args)
}

func makeSubscriberAddressableAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": subscriberAPIVersion,
			"kind":       subscriberKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      subscriberName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": subscriberURI,
				},
			},
		},
	}
}

func makeSubscriberNotAddressableAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": subscriberAPIVersion,
			"kind":       subscriberKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      subscriberName,
			},
		},
	}
}
