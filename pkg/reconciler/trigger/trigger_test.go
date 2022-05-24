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

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/network"
	"knative.dev/pkg/tracker"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing-rabbitmq/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing-rabbitmq/pkg/brokerconfig"
	rabbitduck "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	fakerabbitclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client/fake"
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
	brokerUID   = "broker-test-uid"

	rabbitSecretName         = "test-broker-broker-rabbit"
	rabbitMQBrokerName       = "rabbitbrokerhere"
	rabbitMQBrokerConfigName = "rabbitbrokerconfig"

	triggerName = "test-trigger"
	triggerUID  = "test-trigger-uid"

	rabbitURL = "amqp://localhost:5672/%2f"
	queueName = "test-namespace.test-trigger.broker-test-uid"

	dispatcherImage = "dispatcherimage"

	subscriberURI = "http://example.com/subscriber/"

	pingSourceName                = "test-ping-source"
	testSchedule                  = "*/2 * * * *"
	testContentType               = cloudevents.TextPlain
	testData                      = "data"
	sinkName                      = "testsink"
	dependencyAnnotation          = "{\"kind\":\"PingSource\",\"name\":\"test-ping-source\",\"apiVersion\":\"sources.knative.dev/v1beta2\"}"
	malformedDependencyAnnotation = "\"kind\":\"PingSource\""
	currentGeneration             = 1
	outdatedGeneration            = 0

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
	brokerConfigs := map[string]*duckv1.KReference{
		"rabbitmqClusterConfig": configWithRabbitMQCluster(),
		"rabbitmqBrokerConfig":  configWithRabbitMQBrokerConfig(),
	}
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
			Name: "Broker does not exist",
			Key:  testKey,
			Objects: []runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "test-broker" does not exist`)),
			}},
		},
	}
	for name, config := range brokerConfigs {
		table = append(table, TableTest{
			{
				Name: fmt.Sprintf("%s: Trigger being deleted, not my broker", name),
				Key:  testKey,
				Objects: []runtime.Object{
					broker.NewBroker(brokerName, testNS,
						broker.WithBrokerClass("not-my-broker"),
						broker.WithBrokerConfig(config),
						broker.WithInitBrokerConditions),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithInitTriggerConditions,
						WithTriggerDeleted,
						WithTriggerSubscriberURI(subscriberURI)),
				},
			}, {
				Name: fmt.Sprintf("%s: Not my broker class - no status updates", name),
				Key:  testKey,
				Objects: []runtime.Object{
					broker.NewBroker(brokerName, testNS,
						broker.WithBrokerClass("not-my-broker"),
						broker.WithBrokerConfig(config),
						broker.WithInitBrokerConditions),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithInitTriggerConditions,
						WithTriggerSubscriberURI(subscriberURI)),
				},
			}, {
				Name: fmt.Sprintf("%s: Broker not reconciled yet", name),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config)),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithInitTriggerConditions,
						WithTriggerSubscriberURI(subscriberURI),
						WithTriggerBrokerNotConfigured()),
				},
			}, {
				Name: fmt.Sprintf("%s: Broker not ready yet", name),
				Key:  testKey,
				Objects: []runtime.Object{
					broker.NewBroker(brokerName, testNS,
						broker.WithBrokerClass(brokerClass),
						broker.WithBrokerConfig(config),
						broker.WithInitBrokerConditions,
						broker.WithExchangeFailed("noexchange", "NoExchange")),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithInitTriggerConditions,
						WithTriggerSubscriberURI(subscriberURI),
						WithTriggerBrokerFailed("noexchange", "NoExchange")),
				},
			}, {
				Name: fmt.Sprintf("%s: Creates everything ok", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					triggerWithFilter(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(true)),
				},
				WantCreates: []runtime.Object{
					createDispatcherDeployment(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: triggerWithFilterReady(),
				}},
			}, {
				Name: fmt.Sprintf("%s: Creates everything ok while using Broker DLQ", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBrokerWithDeliverySpec(config),
					triggerWithFilter(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(true)),
				},
				WantCreates: []runtime.Object{
					createDispatcherDeploymentWithRetries(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: triggerWithFilterReady(),
				}},
			},
			{
				Name: fmt.Sprintf("%s: Creates everything ok while using Trigger DLQ", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					triggerWithDeliverySpec(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(true)),
				},
				WantCreates: []runtime.Object{
					createDispatcherDeploymentWithRetries(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: triggerWithDeliverySpecReady(),
				}},
			},
			{
				Name: fmt.Sprintf("%s: Creates queue ok with CRD", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					triggerWithFilter(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
				},
				WantCreates: []runtime.Object{
					createQueue(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: triggerWithQueueNotReady(),
				}},
			}, {
				Name: fmt.Sprintf("%s: Create queue fails with CRD", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					triggerWithFilter(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
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
				Name: fmt.Sprintf("%s: Queue exists, creates binding ok with CRD", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					triggerWithFilter(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
				},
				WantCreates: []runtime.Object{
					createBinding(true),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: triggerWithBindingNotReady(),
				}},
			}, {
				Name: fmt.Sprintf("%s: Queue exists, create binding fails with CRD", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					triggerWithFilter(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "bindings"),
				},
				WantErr: true,
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create bindings"),
				},
				WantCreates: []runtime.Object{
					createBinding(true),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: triggerWithBindingCreateFailure(),
				}},
			}, {
				Name: fmt.Sprintf("%s: Queue, binding exist, creates dispatcher deployment", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					triggerWithFilter(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(true)),
				},
				WantCreates: []runtime.Object{
					createDispatcherDeployment(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: triggerWithFilterReady(),
				}},
			}, {
				Name: fmt.Sprintf("%s: Creates everything with ref", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					makeSubscriberAddressableAsUnstructured(),
					markReady(createQueue()),
					markReady(createBinding(false)),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS)),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
				},
				WantCreates: []runtime.Object{
					createDispatcherDeployment(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
						WithTriggerBrokerReady(),
						WithTriggerDeadLetterSinkNotConfigured(),
						WithTriggerDependencyReady(),
						WithTriggerSubscribed(),
						WithTriggerSubscriberResolvedSucceeded(),
						WithTriggerStatusSubscriberURI(subscriberURI)),
				}},
			}, {
				Name: fmt.Sprintf("%s: Fails to resolve ref", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					makeSubscriberNotAddressableAsUnstructured(),
					markReady(createQueue()),
					markReady(createBinding(false)),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS)),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
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
						WithTriggerDeadLetterSinkNotConfigured(),
						WithTriggerDependencyReady(),
						WithTriggerSubscriberResolvedFailed("Unable to get the Subscriber's URI", `address not set for &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}`)),
				}},
			}, {
				Name: fmt.Sprintf("%s: Deployment creation fails", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI)),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(false)),
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
						WithTriggerDeadLetterSinkNotConfigured(),
						WithTriggerSubscribed(),
						WithTriggerSubscriberResolvedSucceeded(),
						WithTriggerStatusSubscriberURI(subscriberURI),
						WithTriggerDependencyFailed("DeploymentFailure", "inducing failure for create deployments")),
				}},
			}, {
				Name: fmt.Sprintf("%s: Deployment update fails", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI)),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					createDifferentDispatcherDeployment(),
					markReady(createQueue()),
					markReady(createBinding(false)),
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
						WithTriggerDeadLetterSinkNotConfigured(),
						WithTriggerSubscribed(),
						WithTriggerSubscriberResolvedSucceeded(),
						WithTriggerStatusSubscriberURI(subscriberURI),
						WithTriggerDependencyFailed("DeploymentFailure", "inducing failure for update deployments")),
				}},
			}, {
				Name: fmt.Sprintf("%s: Everything ready, nop", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI)),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(false)),
					createDispatcherDeployment(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						WithInitTriggerConditions,
						WithTriggerBrokerReady(),
						WithTriggerDeadLetterSinkNotConfigured(),
						WithTriggerSubscribed(),
						WithTriggerDependencyReady(),
						WithTriggerSubscriberResolvedSucceeded(),
						WithTriggerStatusSubscriberURI(subscriberURI)),
				}},
			}, {
				Name: fmt.Sprintf("%s: Everything ready with filter, nop", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					triggerWithFilter(),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(true)),
					createDispatcherDeployment(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: triggerWithFilterReady(),
				}},
			}, {
				Name: fmt.Sprintf("%s: Dependency doesn't exist", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						WithInitTriggerConditions,
						WithDependencyAnnotation(dependencyAnnotation),
					),
					markReady(createQueue()),
					markReady(createBinding(false)),
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
				Name: fmt.Sprintf("%s: The status of Dependency is False", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					makeFalseStatusPingSource(),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						WithInitTriggerConditions,
						WithDependencyAnnotation(dependencyAnnotation),
					),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(false)),
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						// The first reconciliation will initialize the status conditions.
						WithInitTriggerConditions,
						WithTriggerSubscriberURI(subscriberURI),
						WithDependencyAnnotation(dependencyAnnotation),
						WithTriggerBrokerReady(),
						WithTriggerDependencyFailed("NotFound", ""),
					),
				}},
			}, {
				Name: fmt.Sprintf("%s: The status of Dependency is Unknown", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					makeUnknownStatusCronJobSource(),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						WithInitTriggerConditions,
						WithDependencyAnnotation(dependencyAnnotation),
					),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(false)),
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						// The first reconciliation will initialize the status conditions.
						WithInitTriggerConditions,
						WithDependencyAnnotation(dependencyAnnotation),
						WithTriggerBrokerReady(),
						WithTriggerDependencyUnknown("", ""),
					),
				}},
			}, {
				Name: fmt.Sprintf("%s: Dependency generation not equal", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					makeGenerationNotEqualPingSource(),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						WithInitTriggerConditions,
						WithDependencyAnnotation(dependencyAnnotation),
					),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(false)),
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						// The first reconciliation will initialize the status conditions.
						WithInitTriggerConditions,
						WithDependencyAnnotation(dependencyAnnotation),
						WithTriggerBrokerReady(),
						WithTriggerDependencyUnknown("GenerationNotEqual", fmt.Sprintf("The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", currentGeneration, outdatedGeneration))),
				}},
			}, {
				Name: fmt.Sprintf("%s: Malformed dependency annotation", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						WithInitTriggerConditions,
						WithDependencyAnnotation(malformedDependencyAnnotation),
					),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(false)),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						// The first reconciliation will initialize the status conditions.
						WithInitTriggerConditions,
						WithDependencyAnnotation(malformedDependencyAnnotation),
						WithTriggerBrokerReady(),
						WithTriggerDependencyFailed("ReferenceError", "Unable to unmarshal objectReference from dependency annotation of trigger: invalid character ':' after top-level value")),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "UpdateFailed", "Failed to update status for \"test-trigger\": The provided annotation was not a corev1.ObjectReference: \"\\\"kind\\\":\\\"PingSource\\\"\": metadata.annotations[knative.dev/dependency]\ninvalid character ':' after top-level value"),
				},
			}, {
				Name: fmt.Sprintf("%s: Dependency ready", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					makeReadyPingSource(),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						WithInitTriggerConditions,
						WithDependencyAnnotation(dependencyAnnotation),
					),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(false)),
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
						WithTriggerDeadLetterSinkNotConfigured(),
						WithTriggerSubscribed(),
						WithTriggerStatusSubscriberURI(subscriberURI),
						WithTriggerSubscriberResolvedSucceeded(),
						WithTriggerDependencyReady(),
					),
				}},
			}, {
				Name: fmt.Sprintf("%s: Deployment updated when parallelism value is removed", name),
				Key:  testKey,
				Objects: []runtime.Object{
					ReadyBroker(config),
					NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI)),
					createSecret(rabbitURL),
					createRabbitMQBrokerConfig(),
					markReady(createQueue()),
					markReady(createBinding(false)),
					createDispatcherDeploymentWithParallelism(),
				},
				WantUpdates: []clientgotesting.UpdateActionImpl{{
					Object: createDispatcherDeployment(),
				}},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewTrigger(triggerName, testNS, brokerName,
						WithTriggerUID(triggerUID),
						WithTriggerSubscriberURI(subscriberURI),
						WithInitTriggerConditions,
						WithTriggerBrokerReady(),
						WithTriggerDeadLetterSinkNotConfigured(),
						WithTriggerSubscribed(),
						WithTriggerDependencyReady(),
						WithTriggerSubscriberResolvedSucceeded(),
						WithTriggerStatusSubscriberURI(subscriberURI)),
				}},
			},
		}...)
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
			kubeClientSet:      fakekubeclient.Get(ctx),
			brokerLister:       listers.GetBrokerLister(),
			triggerLister:      listers.GetTriggerLister(),
			deploymentLister:   listers.GetDeploymentLister(),
			sourceTracker:      duck.NewListableTrackerFromTracker(ctx, source.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			addressableTracker: duck.NewListableTrackerFromTracker(ctx, v1addr.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			uriResolver:        resolver.NewURIResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0)),
			brokerClass:        "RabbitMQBroker",
			dispatcherImage:    dispatcherImage,
			rabbitClientSet:    fakerabbitclient.Get(ctx),
			queueLister:        listers.GetQueueLister(),
			bindingLister:      listers.GetBindingLister(),
			rabbit:             rabbit.New(ctx),
			brokerConfig:       brokerconfig.New(ctx),
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

func configWithRabbitMQCluster() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitMQBrokerName,
		Namespace:  testNS,
		Kind:       "RabbitmqCluster",
		APIVersion: "rabbitmq.com/v1beta1",
	}
}

func configWithRabbitMQBrokerConfig() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitMQBrokerConfigName,
		Namespace:  testNS,
		Kind:       "RabbitmqBrokerConfig",
		APIVersion: "eventing.knative.dev/v1alpha1",
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
func ReadyBroker(ref *duckv1.KReference) *eventingv1.Broker {
	return broker.NewBroker(brokerName, testNS,
		broker.WithBrokerUID(brokerUID),
		broker.WithBrokerClass(brokerClass),
		broker.WithInitBrokerConditions,
		broker.WithBrokerConfig(ref),
		broker.WithIngressAvailable(),
		broker.WithSecretReady(),
		broker.WithBrokerAddressURI(brokerAddress),
		broker.WithDLXReady(),
		broker.WithDeadLetterSinkReady(),
		broker.WithExchangeReady())
}

// Create Ready Broker with delivery spec.
func ReadyBrokerWithDeliverySpec(ref *duckv1.KReference) *eventingv1.Broker {
	return broker.NewBroker(brokerName, testNS,
		broker.WithBrokerUID(brokerUID),
		broker.WithBrokerClass(brokerClass),
		broker.WithInitBrokerConditions,
		broker.WithBrokerConfig(ref),
		broker.WithIngressAvailable(),
		broker.WithSecretReady(),
		broker.WithBrokerDelivery(&eventingduckv1.DeliverySpec{}),
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
		Attributes: map[string]string{"type": "dev.knative.sources.ping"},
	}
	return t
}

func triggerWithDeliverySpec() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithTriggerRetry(5, nil, nil),
		WithTriggerSubscriberURI(subscriberURI))
	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: map[string]string{"type": "dev.knative.sources.ping"},
	}
	return t
}

func triggerWithFilterReady() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDeadLetterSinkNotConfigured(),
		WithTriggerDependencyReady(),
		WithTriggerSubscribed(),
		WithTriggerSubscriberResolvedSucceeded(),
		WithTriggerStatusSubscriberURI(subscriberURI))
	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: map[string]string{"type": "dev.knative.sources.ping"},
	}
	return t
}

func triggerWithDeliverySpecReady() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerRetry(5, nil, nil),
		WithTriggerDeadLetterSinkNotConfigured(),
		WithTriggerDependencyReady(),
		WithTriggerSubscribed(),
		WithTriggerSubscriberResolvedSucceeded(),
		WithTriggerStatusSubscriberURI(subscriberURI))
	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: map[string]string{"type": "dev.knative.sources.ping"},
	}
	return t
}

func triggerWithQueueCreateFailure() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithInitTriggerConditions,
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDeadLetterSinkNotConfigured(),
		WithTriggerDependencyReady(),
		WithTriggerDependencyFailed("QueueFailure", `inducing failure for create queues`))

	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: map[string]string{"type": "dev.knative.sources.ping"},
	}
	return t
}

func triggerWithBindingCreateFailure() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithInitTriggerConditions,
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDeadLetterSinkNotConfigured(),
		WithTriggerDependencyReady(),
		WithTriggerDependencyFailed("BindingFailure", `inducing failure for create bindings`))

	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: map[string]string{"type": "dev.knative.sources.ping"},
	}
	return t
}

func triggerWithQueueNotReady() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithInitTriggerConditions,
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDeadLetterSinkNotConfigured(),
		WithTriggerDependencyReady(),
		WithTriggerDependencyFailed("QueueFailure", `Queue "t.test-namespace.test-trigger.test-trigger-uid" is not ready`))

	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: map[string]string{"type": "dev.knative.sources.ping"},
	}
	return t
}

func triggerWithBindingNotReady() *eventingv1.Trigger {
	t := NewTrigger(triggerName, testNS, brokerName,
		WithTriggerUID(triggerUID),
		WithInitTriggerConditions,
		WithTriggerSubscriberURI(subscriberURI),
		WithTriggerBrokerReady(),
		WithTriggerDeadLetterSinkNotConfigured(),
		WithTriggerDependencyReady(),
		WithTriggerDependencyFailed("BindingFailure", `Binding "t.test-namespace.test-trigger.test-trigger-uid" is not ready`))

	t.Spec.Filter = &eventingv1.TriggerFilter{
		Attributes: map[string]string{"type": "dev.knative.sources.ping"},
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

func createRabbitMQBrokerConfig() *v1alpha1.RabbitmqBrokerConfig {
	return &v1alpha1.RabbitmqBrokerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rabbitMQBrokerConfigName,
			Namespace: testNS,
		},
		Spec: v1alpha1.RabbitmqBrokerConfigSpec{
			RabbitmqClusterReference: &rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitMQBrokerName,
				Namespace: testNS,
			},
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

func createDispatcherDeploymentWithRetries() *appsv1.Deployment {
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
		Delivery:           &eventingduckv1.DeliverySpec{},
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

func createDispatcherDeploymentWithParallelism() *appsv1.Deployment {
	args := &resources.DispatcherArgs{
		Trigger: &eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerName,
				Namespace: testNS,
				UID:       triggerUID,
				Annotations: map[string]string{
					resources.ParallelismAnnotation: "10",
				},
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
		"eventing.knative.dev/broker":  brokerName,
		"eventing.knative.dev/trigger": triggerName,
	}
	t := triggerWithFilter()
	return &rabbitv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "t.test-namespace.test-trigger.test-trigger-uid",
			Namespace: testNS,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(t),
			},
			Labels: labels,
		},
		Spec: rabbitv1beta1.QueueSpec{
			Name:    queueName,
			Vhost:   "/",
			Durable: true,
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitMQBrokerName,
				Namespace: testNS,
			},
		},
	}
}

func createBinding(withFilter bool) *rabbitv1beta1.Binding {
	bindingName := fmt.Sprintf("t.%s.%s.test-trigger-uid", testNS, triggerName)
	labels := map[string]string{
		"eventing.knative.dev/broker":  brokerName,
		"eventing.knative.dev/trigger": triggerName,
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
			Destination:     queueName,
			Source:          "b.test-namespace.test-broker.broker-test-uid",
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitMQBrokerName,
				Namespace: testNS,
			},
			// We need to know if we need to include the filter in the
			Arguments: getTriggerArguments(withFilter),
		},
	}
}

func getTriggerArguments(withFilter bool) *runtime.RawExtension {
	arguments := map[string]string{
		"x-knative-trigger": triggerName,
		"x-match":           "all",
	}
	if withFilter {
		arguments["type"] = "dev.knative.sources.ping"
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}

func markReady(r runtime.Object) runtime.Object {
	ready := rabbitv1beta1.Condition{Status: corev1.ConditionTrue}
	switch v := r.(type) {
	case *rabbitv1beta1.Binding:
		v.Status.Conditions = append(v.Status.Conditions, ready)
	case *rabbitv1beta1.Exchange:
		v.Status.Conditions = append(v.Status.Conditions, ready)
	case *rabbitv1beta1.Queue:
		v.Status.Conditions = append(v.Status.Conditions, ready)
	case *rabbitv1beta1.Policy:
		v.Status.Conditions = append(v.Status.Conditions, ready)
	default:
		panic("unknown type")
	}
	return r
}
