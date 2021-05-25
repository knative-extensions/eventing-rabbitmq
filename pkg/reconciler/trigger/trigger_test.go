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
	"encoding/json"
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/network"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	clientgotesting "k8s.io/client-go/testing"
	rabbitduck "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	fakerabbitclient "knative.dev/eventing-rabbitmq/pkg/client/injection/rabbitmq.com/client/fake"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker"
	brokerresources "knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	sourcesv1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1addr "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/source"
	v1a1addr "knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	v1b1addr "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"

	rtlisters "knative.dev/eventing-rabbitmq/pkg/reconciler/testing"
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	rtv1beta2 "knative.dev/eventing/pkg/reconciler/testing/v1beta2"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	systemNS    = "knative-testing"
	testNS      = "test-namespace"
	brokerClass = "RabbitMQBroker"
	brokerName  = "test-broker"

	rabbitSecretName   = "test-broker-broker-rabbit"
	rabbitMQBrokerName = "rabbitbrokerhere"

	triggerName = "test-trigger"
	triggerUID  = "test-trigger-uid"

	rabbitURL = "amqp://localhost:5672/%2f"
	queueName = "test-namespace.test-trigger"

	dispatcherImage = "dispatcherimage"

	subscriberURI = "http://example.com/subscriber/"

	pingSourceName       = "test-ping-source"
	testSchedule         = "*/2 * * * *"
	testContentType      = cloudevents.TextPlain
	testData             = "data"
	sinkName             = "testsink"
	dependencyAnnotation = "{\"kind\":\"PingSource\",\"name\":\"test-ping-source\",\"apiVersion\":\"sources.knative.dev/v1beta2\"}"
	currentGeneration    = 1
	outdatedGeneration   = 0

	subscriberKind    = "Service"
	subscriberName    = "subscriber-name"
	subscriberGroup   = "serving.knative.dev"
	subscriberVersion = "v1"
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
	sinkDNS = network.GetServiceHostname("sink", "mynamespace")
	sinkURI = "http://" + sinkDNS

	brokerAddress = &apis.URL{
		Scheme: "http",
		Host:   network.GetServiceHostname(ingressServiceName, systemNS),
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
		}, {
			Name: "Creates everything ok",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				triggerWithFilter(),
				createSecret(rabbitURL),
			},
			WantCreates: []runtime.Object{
				createDispatcherDeployment(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: triggerWithFilterReady(),
			}},
		}, {
			Name: "Creates queue ok with CRD",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBrokerWithRabbitBroker(),
				triggerWithFilter(),
				createSecret(rabbitURL),
			},
			WantCreates: []runtime.Object{
				createQueue(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: triggerWithQueueNotReady(),
			}},
		}, {
			Name: "Create queue fails with CRD",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBrokerWithRabbitBroker(),
				triggerWithFilter(),
				createSecret(rabbitURL),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "queues"),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create queues"),
			},
			WantCreates: []runtime.Object{
				createQueue(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: triggerWithQueueCreateFailure(),
			}},
		}, {
			Name: "Queue exists, creates binding ok with CRD",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBrokerWithRabbitBroker(),
				triggerWithFilter(),
				createSecret(rabbitURL),
				createReadyQueue(),
			},
			WantCreates: []runtime.Object{
				createBinding(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: triggerWithBindingNotReady(),
			}},
		}, {
			Name: "Queue exists, create binding fails with CRD",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBrokerWithRabbitBroker(),
				triggerWithFilter(),
				createSecret(rabbitURL),
				createReadyQueue(),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "bindings"),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create bindings"),
			},
			WantCreates: []runtime.Object{
				createBinding(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: triggerWithBindingCreateFailure(),
			}},
		}, {
			Name: "Queue, binding exist, creates dispatcher deployment",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBrokerWithRabbitBroker(),
				triggerWithFilter(),
				createSecret(rabbitURL),
				createReadyQueue(),
				createReadyBinding(),
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
			WantEvents: []string{
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
			WantEvents: []string{
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
			WantEvents: []string{
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
			WantEvents: []string{
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
	table.Test(t, rtlisters.MakeFactory(func(ctx context.Context, listers *rtlisters.Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = source.WithDuck(ctx)
		ctx = rabbitduck.WithDuck(ctx)
		r := &Reconciler{
			eventingClientSet:  fakeeventingclient.Get(ctx),
			dynamicClientSet:   fakedynamicclient.Get(ctx),
			kubeClientSet:      fakekubeclient.Get(ctx),
			brokerLister:       listers.GetBrokerLister(),
			triggerLister:      listers.GetTriggerLister(),
			deploymentLister:   listers.GetDeploymentLister(),
			sourceTracker:      duck.NewListableTracker(ctx, source.Get, func(types.NamespacedName) {}, 0),
			addressableTracker: duck.NewListableTracker(ctx, v1addr.Get, func(types.NamespacedName) {}, 0),
			uriResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			brokerClass:        "RabbitMQBroker",
			dispatcherImage:    dispatcherImage,
			rabbitClientSet:    fakerabbitclient.Get(ctx),
			queueLister:        listers.GetQueueLister(),
			bindingLister:      listers.GetBindingLister(),
		}
		return trigger.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetTriggerLister(),
			controller.GetEventRecorder(ctx),
			r,
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

func configForRabbitOperator() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitMQBrokerName,
		Namespace:  testNS,
		Kind:       "RabbitmqCluster",
		APIVersion: "rabbitmq.com/v1beta1",
	}
}

func makeFalseStatusPingSource() *sourcesv1beta2.PingSource {
	return rtv1beta2.NewPingSource(pingSourceName, testNS, rtv1beta2.WithPingSourceSinkNotFound)
}

func makeUnknownStatusCronJobSource() *sourcesv1beta2.PingSource {
	cjs := rtv1beta2.NewPingSource(pingSourceName, testNS)
	cjs.Status.InitializeConditions()
	return cjs
}

func makeGenerationNotEqualPingSource() *sourcesv1beta2.PingSource {
	c := makeFalseStatusPingSource()
	c.Generation = currentGeneration
	c.Status.ObservedGeneration = outdatedGeneration
	return c
}

func makeReadyPingSource() *sourcesv1beta2.PingSource {
	u, _ := apis.ParseURL(sinkURI)
	return rtv1beta2.NewPingSource(pingSourceName, testNS,
		rtv1beta2.WithPingSourceSpec(sourcesv1beta2.PingSourceSpec{
			Schedule:    testSchedule,
			ContentType: testContentType,
			Data:        testData,
			SourceSpec: duckv1.SourceSpec{
				Sink: brokerDestv1,
			},
		}),
		rtv1beta2.WithInitPingSourceConditions,
		rtv1beta2.WithPingSourceDeployed,
		rtv1beta2.WithPingSourceCloudEventAttributes,
		rtv1beta2.WithPingSourceSink(u),
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
		broker.WithDLXReady(),
		broker.WithDeadLetterSinkReady(),
		broker.WithExchangeReady())
}

// Create Ready Broker with proper annotations using the RabbitmqCluster
func ReadyBrokerWithRabbitBroker() *eventingv1.Broker {
	return broker.NewBroker(brokerName, testNS,
		broker.WithBrokerClass(brokerClass),
		broker.WithInitBrokerConditions,
		broker.WithBrokerConfig(configForRabbitOperator()),
		broker.WithIngressAvailable(),
		broker.WithSecretReady(),
		broker.WithBrokerAddressURI(brokerAddress),
		broker.WithDLXReady(),
		broker.WithDeadLetterSinkReady(),
		broker.WithExchangeReady())
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

func triggerWithQueueCreateFailure() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithInitTriggerConditions,
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDependencyReady(),
		WithTriggerDependencyFailed("QueueFailure", `inducing failure for create queues`))

	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: eventingv1.TriggerFilterAttributes(map[string]string{"type": "dev.knative.sources.ping"}),
	}
	return t
}

func triggerWithBindingCreateFailure() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithInitTriggerConditions,
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDependencyReady(),
		WithTriggerDependencyFailed("BindingFailure", `inducing failure for create bindings`))

	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: eventingv1.TriggerFilterAttributes(map[string]string{"type": "dev.knative.sources.ping"}),
	}
	return t
}

func triggerWithQueueNotReady() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithInitTriggerConditions,
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDependencyReady(),
		WithTriggerDependencyFailed("QueueFailure", `Queue "test-namespace.test-trigger" is not ready`))

	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: eventingv1.TriggerFilterAttributes(map[string]string{"type": "dev.knative.sources.ping"}),
	}
	return t
}

func triggerWithBindingNotReady() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithInitTriggerConditions,
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDependencyReady(),
		WithTriggerDependencyFailed("BindingFailure", `Binding "test-namespace.test-trigger" is not ready`))

	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: eventingv1.TriggerFilterAttributes(map[string]string{"type": "dev.knative.sources.ping"}),
	}
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
		QueueName:          queueName,
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
		QueueName:          queueName,
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

func createQueue() *rabbitv1beta1.Queue {
	labels := map[string]string{
		eventing.BrokerLabelKey:   brokerName,
		resources.TriggerLabelKey: triggerName,
	}
	b := ReadyBrokerWithRabbitBroker()
	t := triggerWithFilter()
	return &rabbitv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      queueName,
			Namespace: testNS,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(t),
			},
			Labels: labels,
		},
		Spec: rabbitv1beta1.QueueSpec{
			Name:    queueName,
			Durable: true,
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitMQBrokerName,
			},
			Arguments: &runtime.RawExtension{
				Raw: []byte(`{"x-dead-letter-exchange":"` + brokerresources.ExchangeName(b, true) + `"}`),
			},
		},
	}
}

func createReadyQueue() *rabbitv1beta1.Queue {
	q := createQueue()
	q.Status = rabbitv1beta1.QueueStatus{
		Conditions: []rabbitv1beta1.Condition{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}
	return q
}

func createBinding() *rabbitv1beta1.Binding {
	bindingName := fmt.Sprintf("%s.%s", testNS, triggerName)

	labels := map[string]string{
		eventing.BrokerLabelKey:   brokerName,
		resources.TriggerLabelKey: triggerName,
	}
	trigger := triggerWithFilter()
	return &rabbitv1beta1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      bindingName,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(trigger),
			},
			Labels: labels,
		},
		Spec: rabbitv1beta1.BindingSpec{
			Vhost:           "/",
			DestinationType: "queue",
			Destination:     bindingName,
			Source:          "test-namespace.test-broker",
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitMQBrokerName,
			},
			Arguments: getTriggerArguments(),
		},
	}
}

func createReadyBinding() *rabbitv1beta1.Binding {
	b := createBinding()
	b.Status = rabbitv1beta1.BindingStatus{
		Conditions: []rabbitv1beta1.Condition{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}
	return b

}

func getTriggerArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":           "all",
		"x-knative-trigger": triggerName,
		"type":              "dev.knative.sources.ping",
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}
