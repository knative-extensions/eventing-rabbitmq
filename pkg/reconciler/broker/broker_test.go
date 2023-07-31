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
	"encoding/json"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	clientgotesting "k8s.io/client-go/testing"
	rabbitmqduck "knative.dev/eventing-rabbitmq/pkg/apis/duck/v1beta1"
	"knative.dev/eventing-rabbitmq/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing-rabbitmq/pkg/brokerconfig"
	rabbitduck "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	fakerabbitclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client/fake"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
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
	"knative.dev/pkg/kmeta"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	rtlisters "knative.dev/eventing-rabbitmq/pkg/reconciler/testing"

	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	rt "knative.dev/eventing/pkg/reconciler/testing/v1"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"

	. "knative.dev/pkg/reconciler/testing"
)

const (
	brokerClass = "RabbitMQBroker"
	testNS      = "test-namespace"
	brokerName  = "test-broker"
	brokerUID   = "broker-test-uid"

	rabbitSecretName          = "test-secret"
	rabbitBrokerSecretName    = "test-broker-broker-rabbit"
	rabbitmqClusterSecretName = "rabbitmqclustersecret"
	rabbitURL                 = "amqp://localhost:5672/%2f"
	rabbitmqClusterURL        = "amqp://myusername:mypassword@rabbitmqsvc.test-namespace.svc.cluster.local:5672"
	rabbitMQBrokerName        = "rabbitbrokerhere"
	rabbitMQBrokerConfigName  = "rabbitbrokerconfig"
	rabbitMQVhost             = "test-vhost"
	ingressImage              = "ingressimage"

	deadLetterSinkKind       = "Service"
	deadLetterSinkName       = "badsink"
	deadLetterSinkAPIVersion = "v1"

	dispatcherImage = "dispatcherimage"
)

var (
	TrueValue = true

	testKey = fmt.Sprintf("%s/%s", testNS, brokerName)

	ingressServiceName = "test-broker-broker-ingress"
	brokerAddress      = &apis.URL{
		Scheme: "http",
		Host:   network.GetServiceHostname(ingressServiceName, testNS),
	}
	deadLetterSinkAddress = &apis.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/subscriber/",
	}
	DLSAddressable = &duckv1.Addressable{
		URL: deadLetterSinkAddress,
		// CACerts: , still to be implemented
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
	deliveryUnresolvableDeadLetterSink = &eventingduckv1.DeliverySpec{
		DeadLetterSink: &duckv1.Destination{
			Ref: &duckv1.KReference{
				Name:       deadLetterSinkName,
				Kind:       deadLetterSinkKind,
				APIVersion: deadLetterSinkAPIVersion,
			},
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
	_ = rabbitmqduck.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {
	table := TableTest{{
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
		Name: "nil config",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerClass),
				WithInitBrokerConditions),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Broker.Spec.Config is required`),
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
			Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-broker": missing field(s): spec.config.name`),
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
		Name: "Invalid spec",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerUID(brokerUID),
				WithBrokerClass(brokerClass),
				WithBrokerConfig(invalidConfigForRabbitOperator()),
				WithInitBrokerConditions),
		},
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerUID(brokerUID),
				WithBrokerClass(brokerClass),
				WithInitBrokerConditions,
				WithBrokerConfig(invalidConfigForRabbitOperator()),
				WithExchangeFailed("ExchangeCredentialsUnavailable", `Failed to get arguments for creating exchange: Broker.Spec.Config configuration not supported, only [kind: RabbitmqCluster, apiVersion: rabbitmq.com/v1beta1] and [Kind: RabbitmqBrokerConfig, apiVersion: eventing.knative.dev/v1alpha1]`)),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Broker.Spec.Config configuration not supported, only [kind: RabbitmqCluster, apiVersion: rabbitmq.com/v1beta1] and [Kind: RabbitmqBrokerConfig, apiVersion: eventing.knative.dev/v1alpha1]`),
		},
	}}

	brokerConfigs := map[string]*duckv1.KReference{
		"rabbitmqClusterConfig": rabbitmqClusterConfigRef(),
		"rabbitmqBrokerConfig":  rabbitmqBrokerConfigRef(),
	}

	for configName, config := range brokerConfigs {
		table = append(table, TableTest{
			{
				Name: fmt.Sprintf("%s: Broker is being deleted", config.Name),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions,
						WithBrokerDeletionTimestamp),
				},
			}, {
				Name: fmt.Sprintf("%s: Broker does not exist", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQBrokerConfig(""),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeCredentialsUnavailable", `Failed to get arguments for creating exchange: rabbitmqclusters.rabbitmq.com "rabbitbrokerhere" not found`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `rabbitmqclusters.rabbitmq.com "rabbitbrokerhere" not found`),
				},
			}, {
				Name: fmt.Sprintf("%s: Broker not ready", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createRabbitMQClusterMissingServiceRef(),
					createRabbitMQBrokerConfig(""),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeCredentialsUnavailable", `Failed to get arguments for creating exchange: rabbit "test-namespace/rabbitbrokerhere" not ready`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `rabbit "test-namespace/rabbitbrokerhere" not ready`),
				},
			}, {
				Name: fmt.Sprintf("%s: Broker ready, no secret", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeCredentialsUnavailable", `Failed to get arguments for creating exchange: secrets "rabbitmqclustersecret" not found`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `secrets "rabbitmqclustersecret" not found`),
				},
			}, {
				Name: fmt.Sprintf("%s: Broker ready, secret missing username", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqClusterNoUser(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeCredentialsUnavailable", `Failed to get arguments for creating exchange: rabbit Secret missing key username`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `rabbit Secret missing key username`),
				},
			}, {
				Name: fmt.Sprintf("%s: Broker ready, secret missing password", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqClusterNoPassword(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeCredentialsUnavailable", `Failed to get arguments for creating exchange: rabbit Secret missing key password`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `rabbit Secret missing key password`),
				},
			}, {
				Name: fmt.Sprintf("%s: Exchange CRD create fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "exchanges"),
				},
				WantErr: true,
				WantCreates: []runtime.Object{
					createExchange(false, ""),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeFailure", `Failed to reconcile exchange "b.test-namespace.test-broker.broker-test-uid": inducing failure for create exchanges`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create exchanges`),
				},
			}, {
				Name: fmt.Sprintf("%s: Exchange create, creates Exchange CRD, not ready", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
				},
				WantCreates: []runtime.Object{
					createExchange(false, ""),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeFailure", `exchange "b.test-namespace.test-broker.broker-test-uid" is not ready`)),
				}},
			}, {
				Name: fmt.Sprintf("%s: Exchange exists, create DLX CRD, not ready", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerDelivery(delivery),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
				},
				WantCreates: []runtime.Object{
					createExchange(true, ""),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerDelivery(delivery),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeFailure", `DLX exchange "b.test-namespace.test-broker.dlx.broker-test-uid" is not ready`)),
				}},
			}, {
				Name: fmt.Sprintf("%s: Exchange exists, create DLX exchange fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerDelivery(delivery),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
				},
				WantCreates: []runtime.Object{
					createExchange(true, ""),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "exchanges"),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerDelivery(delivery),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeFailed("ExchangeFailure", `Failed to reconcile DLX exchange "b.test-namespace.test-broker.dlx.broker-test-uid": inducing failure for create exchanges`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create exchanges`),
				},
			}, {
				Name: fmt.Sprintf("%s: Both exchanges exist, create dead letter queue CRD fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery)),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
				},
				WantCreates: []runtime.Object{
					createQueue(true, config, ""),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "queues"),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithBrokerDelivery(delivery),
						WithExchangeReady(),
						WithDLXFailed("QueueFailure", `Failed to reconcile Dead Letter Queue "b.test-namespace.test-broker.dlq.broker-test-uid" : inducing failure for create queues`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create queues`),
				},
			}, {
				Name: fmt.Sprintf("%s: Both exchanges exist, dlq delivery specified, creates dead letter queue CRD", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery)),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
				},
				WantCreates: []runtime.Object{
					createQueue(true, config, ""),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery),
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXFailed("QueueFailure", `Dead Letter Queue "b.test-namespace.test-broker.dlq.broker-test-uid" is not ready`)),
				}},
			}, {
				Name: fmt.Sprintf("%s: Both exchanges exist, dlq delivery specified, queue exists, creates binding CRD", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery)),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
				},
				WantCreates: []runtime.Object{
					createBinding(true, ""),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery),
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXReady(),
						WithDeadLetterSinkFailed("DLQ binding", `DLQ binding "b.test-namespace.test-broker.dlq.broker-test-uid" is not ready`)),
				}},
			}, {
				Name: fmt.Sprintf("%s: Both exchanges exist, queue exists, create binding CRD fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery)),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
				},
				WantCreates: []runtime.Object{
					createBinding(true, ""),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "bindings"),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery),
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXReady(),
						WithDeadLetterSinkFailed("DLQ binding", `Failed to reconcile DLQ binding "b.test-namespace.test-broker.dlq.broker-test-uid" : inducing failure for create bindings`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create bindings`),
				},
			}, {
				Name: fmt.Sprintf("%s: Both exchanges exist, queue exists, binding exists, creates policy fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery)),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
				},
				WantCreates: []runtime.Object{
					createPolicy(""),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "policies"),
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery),
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXReady(),
						WithDeadLetterSinkFailed("PolicyFailure", `Failed to reconcile RabbitMQ Policy "b.test-namespace.test-broker.dlq.broker-test-uid" : inducing failure for create policies`)),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create policies`),
				},
			}, {
				Name: fmt.Sprintf("%s: Both exchanges exist, queue exists, binding exists, policy not ready", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery)),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
				},
				WantCreates: []runtime.Object{
					createPolicy(""),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerDelivery(delivery),
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXReady(),
						WithDeadLetterSinkFailed("PolicyFailure", `RabbitMQ Policy "b.test-namespace.test-broker.dlq.broker-test-uid" is not ready`)),
				}},
			}, {
				Name: fmt.Sprintf("%s: Both exchanges exist, creates secret, deployment, and service, endpoints not ready", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
				},
				WantCreates: []runtime.Object{
					createBrokerSecretFromRabbitmqCluster(),
					createIngressDeployment(),
					createIngressService(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithIngressFailed("ServiceFailure", `Failed to reconcile service: endpoints "test-broker-broker-ingress" not found`),
						WithSecretReady(),
						WithExchangeReady(),
						WithDLXNotConfigured(),
						WithDeadLetterSinkNotConfigured()),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "test-broker-broker-ingress" not found`),
				},
				WantErr: true,
			}, {
				Name: fmt.Sprintf("%s: Both exchanges exist, creates secret, deployment, and service, endpoints ready", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				},
				WantCreates: []runtime.Object{
					createBrokerSecretFromRabbitmqCluster(),
					createIngressDeployment(),
					createIngressService(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithBrokerAddressURI(brokerAddress),
						WithIngressAvailable(),
						WithSecretReady(),
						WithExchangeReady(),
						WithDLXNotConfigured(),
						WithDeadLetterSinkResolvedNotConfigured(),
						WithDeadLetterSinkNotConfigured()),
				}},
			}, {
				Name: fmt.Sprintf("%s: Secret create fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "secrets"),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXNotConfigured(),
						WithDeadLetterSinkNotConfigured(),
						WithSecretFailed("SecretFailure", `Failed to reconcile secret: inducing failure for create secrets`)),
				}},
				WantCreates: []runtime.Object{
					createExchangeSecret(""),
				},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create secrets`),
				},
				WantErr: true,
			}, {
				Name: fmt.Sprintf("%s: Secret update fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					createReadyPolicy(""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
					createDifferentExchangeSecret(),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("update", "secrets"),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXNotConfigured(),
						WithDeadLetterSinkNotConfigured(),
						WithSecretFailed("SecretFailure", `Failed to reconcile secret: inducing failure for update secrets`)),
				}},
				WantUpdates: []clientgotesting.UpdateActionImpl{{
					Object: createExchangeSecret(""),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for update secrets`),
				},
				WantErr: true,
			}, {
				Name: fmt.Sprintf("%s: Deployment create fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					createReadyPolicy(""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
					createSecret(rabbitURL),
					createExchangeSecret(""),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "deployments"),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXNotConfigured(),
						WithDeadLetterSinkNotConfigured(),
						WithSecretReady(),
						WithIngressFailed("DeploymentFailure", `Failed to reconcile deployment: inducing failure for create deployments`)),
				}},
				WantCreates: []runtime.Object{
					createIngressDeployment(),
				},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create deployments`),
				},
				WantErr: true,
			}, {
				Name: fmt.Sprintf("%s: Deployment update fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					createReadyPolicy(""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
					createExchangeSecret(""),
					createDifferentIngressDeployment(),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("update", "deployments"),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXNotConfigured(),
						WithDeadLetterSinkNotConfigured(),
						WithSecretReady(),
						WithIngressFailed("DeploymentFailure", `Failed to reconcile deployment: inducing failure for update deployments`)),
				}},
				WantUpdates: []clientgotesting.UpdateActionImpl{{
					Object: createIngressDeployment(),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for update deployments`),
				},
				WantErr: true,
			}, {
				Name: fmt.Sprintf("%s: Service create fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					createReadyPolicy(""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
					createExchangeSecret(""),
					createIngressDeployment(),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("create", "services"),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXNotConfigured(),
						WithDeadLetterSinkNotConfigured(),
						WithSecretReady(),
						WithIngressFailed("ServiceFailure", `Failed to reconcile service: inducing failure for create services`)),
				}},
				WantCreates: []runtime.Object{
					createIngressService(),
				},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for create services`),
				},
				WantErr: true,
			}, {
				Name: fmt.Sprintf("%s: Service update fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					createReadyPolicy(""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
					createExchangeSecret(""),
					createIngressDeployment(),
					createDifferentIngressService(),
				},
				WithReactors: []clientgotesting.ReactionFunc{
					InduceFailure("update", "services"),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithExchangeReady(),
						WithDLXNotConfigured(),
						WithDeadLetterSinkNotConfigured(),
						WithSecretReady(),
						WithIngressFailed("ServiceFailure", `Failed to reconcile service: inducing failure for update services`)),
				}},
				WantUpdates: []clientgotesting.UpdateActionImpl{{
					Object: createIngressService(),
				}},
				WantEvents: []string{
					Eventf(corev1.EventTypeWarning, "InternalError", `inducing failure for update services`),
				},
				WantErr: true,
			}, {
				Name: fmt.Sprintf("%s: Exchange created with DLQ dispatcher created", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithBrokerDelivery(delivery),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					createReadyPolicy(""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
					createExchangeSecret(""),
					createIngressDeployment(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithBrokerDelivery(delivery),
						WithIngressAvailable(),
						WithDLXReady(),
						WithDeadLetterSinkReady(),
						WithDeadLetterSinkResolvedSucceeded(AddressableFromDestination(delivery.DeadLetterSink)),
						WithSecretReady(),
						WithBrokerAddressURI(brokerAddress),
						WithExchangeReady()),
				}},
				WantCreates: []runtime.Object{
					createIngressService(),
					createDispatcherDeployment(),
				},
				WantErr: false,
			}, {
				Name: fmt.Sprintf("%s: broker with custom resource requests. resource requirements gets used for both ingress and dlq dispatcher", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithBrokerDelivery(delivery),
						WithInitBrokerConditions,
						WithAnnotation(utils.CPURequestAnnotation, "500m"),
						WithAnnotation(utils.CPULimitAnnotation, "1000m"),
						WithAnnotation(utils.MemoryRequestAnnotation, "500Mi"),
						WithAnnotation(utils.MemoryLimitAnnotation, "1000Mi")),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					createReadyPolicy(""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
					createExchangeSecret(""),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithBrokerDelivery(delivery),
						WithIngressAvailable(),
						WithDLXReady(),
						WithDeadLetterSinkReady(),
						WithDeadLetterSinkResolvedSucceeded(AddressableFromDestination(delivery.DeadLetterSink)),
						WithSecretReady(),
						WithBrokerAddressURI(brokerAddress),
						WithAnnotation(utils.CPURequestAnnotation, "500m"),
						WithAnnotation(utils.CPULimitAnnotation, "1000m"),
						WithAnnotation(utils.MemoryRequestAnnotation, "500Mi"),
						WithAnnotation(utils.MemoryLimitAnnotation, "1000Mi"),
						WithExchangeReady()),
				}},
				WantCreates: []runtime.Object{
					createIngressDeployment(
						func(args *resources.IngressArgs) {
							args.ResourceRequirements = corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("500Mi")},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("1000Mi")},
							}
						},
					),
					createIngressService(),
					createDispatcherDeployment(
						func(args *resources.DispatcherArgs) {
							args.ResourceRequirements = corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("500Mi")},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("1000Mi")},
							}
						},
					),
				},
				WantErr: false,
			}, {
				Name: fmt.Sprintf("%s: Exchange created with unresolvable delivery, DLQ dispatcher fails", configName),
				Key:  testKey,
				Objects: []runtime.Object{
					NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithBrokerConfig(config),
						WithBrokerDelivery(deliveryUnresolvableDeadLetterSink),
						WithInitBrokerConditions),
					createSecretForRabbitmqCluster(),
					createRabbitMQCluster(""),
					createRabbitMQBrokerConfig(""),
					createReadyExchange(false, ""),
					createReadyExchange(true, ""),
					createReadyQueue(true, config, ""),
					createReadyBinding(true, ""),
					createReadyPolicy(""),
					rt.NewEndpoints(ingressServiceName, testNS,
						rt.WithEndpointsLabels(IngressLabels()),
						rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
					createExchangeSecret(""),
					createIngressDeployment(),
					createIngressService(),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewBroker(brokerName, testNS,
						WithBrokerUID(brokerUID),
						WithBrokerClass(brokerClass),
						WithInitBrokerConditions,
						WithBrokerConfig(config),
						WithBrokerDelivery(deliveryUnresolvableDeadLetterSink),
						WithIngressAvailable(),
						WithDLXReady(),
						WithDeadLetterSinkReady(),
						WithDeadLetterSinkResolvedFailed(
							"Unable to get the DeadLetterSink's Addressable",
							fmt.Sprintf(`failed to get object %s/%s: services "%s" not found`, testNS, deadLetterSinkName, deadLetterSinkName),
						),
						WithSecretReady(),
						WithBrokerAddressURI(brokerAddress),
						WithExchangeReady()),
				}},
				WantEvents: []string{
					Eventf(
						corev1.EventTypeWarning,
						"InternalError",
						fmt.Sprintf(`failed to get object %s/%s: services "%s" not found`, testNS, deadLetterSinkName, deadLetterSinkName),
					),
				},
				WantErr: true,
			},
		}...)
	}

	table = append(table, TableTest{{
		Name: "Created broker with DLQ and no Cluster Reference namespace",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerUID(brokerUID),
				WithBrokerClass(brokerClass),
				WithBrokerConfig(rabbitmqClusterConfigRefNoNS()),
				WithBrokerDelivery(delivery),
				WithInitBrokerConditions),
			createSecretForRabbitmqCluster(),
			createRabbitMQCluster(""),
			createRabbitMQBrokerConfig(""),
			createReadyExchange(false, ""),
			createReadyExchange(true, ""),
			createReadyQueue(true, rabbitmqClusterConfigRefNoNS(), ""),
			createReadyBinding(true, ""),
			createReadyPolicy(""),
			rt.NewEndpoints(ingressServiceName, testNS,
				rt.WithEndpointsLabels(IngressLabels()),
				rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			createExchangeSecret(""),
			createIngressDeployment(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerUID(brokerUID),
				WithBrokerClass(brokerClass),
				WithInitBrokerConditions,
				WithBrokerConfig(rabbitmqClusterConfigRefNoNS()),
				WithBrokerDelivery(delivery),
				WithIngressAvailable(),
				WithDLXReady(),
				WithDeadLetterSinkReady(),
				WithDeadLetterSinkResolvedSucceeded(AddressableFromDestination(delivery.DeadLetterSink)),
				WithSecretReady(),
				WithBrokerAddressURI(brokerAddress),
				WithExchangeReady()),
		}},
		WantCreates: []runtime.Object{
			createIngressService(),
			createDispatcherDeployment(),
		},
		WantErr: false,
	}, {
		Name: "Created broker in vhost",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerUID(brokerUID),
				WithBrokerClass(brokerClass),
				WithBrokerConfig(rabbitmqBrokerConfigRef()),
				WithDLXNotConfigured(),
				WithDeadLetterSinkNotConfigured(),
				WithDeadLetterSinkResolvedNotConfigured(),
				WithInitBrokerConditions),
			createSecretForRabbitmqCluster(),
			createRabbitMQCluster(rabbitMQVhost),
			createRabbitMQBrokerConfig(rabbitMQVhost),
			createReadyExchange(false, rabbitMQVhost),
			rt.NewEndpoints(ingressServiceName, testNS,
				rt.WithEndpointsLabels(IngressLabels()),
				rt.WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			createExchangeSecret(rabbitMQVhost),
			createIngressDeployment(),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: createExchangeSecret(""),
		}, {
			Object: createIngressDeployment(WithVhost),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerUID(brokerUID),
				WithBrokerClass(brokerClass),
				WithInitBrokerConditions,
				WithBrokerConfig(rabbitmqBrokerConfigRef()),
				WithIngressAvailable(),
				WithDLXNotConfigured(),
				WithDeadLetterSinkNotConfigured(),
				WithDeadLetterSinkResolvedNotConfigured(),
				WithSecretReady(),
				WithBrokerAddressURI(brokerAddress),
				WithExchangeReady()),
		}},
		WantCreates: []runtime.Object{
			createIngressService(),
		},
		WantErr: false,
	}}...)

	logger := logtesting.TestLogger(t)
	table.Test(t, rtlisters.MakeFactory(func(ctx context.Context, listers *rtlisters.Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = conditions.WithDuck(ctx)
		ctx = rabbitduck.WithDuck(ctx)
		eventingv1.RegisterAlternateBrokerConditionSet(rabbitBrokerCondSet)
		r := &Reconciler{
			eventingClientSet:  fakeeventingclient.Get(ctx),
			kubeClientSet:      fakekubeclient.Get(ctx),
			endpointsLister:    listers.GetEndpointsLister(),
			serviceLister:      listers.GetServiceLister(),
			secretLister:       listers.GetSecretLister(),
			deploymentLister:   listers.GetDeploymentLister(),
			kresourceTracker:   duck.NewListableTrackerFromTracker(ctx, conditions.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			addressableTracker: duck.NewListableTrackerFromTracker(ctx, v1a1addr.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			uriResolver:        resolver.NewURIResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0)),
			brokerClass:        "RabbitMQBroker",
			ingressImage:       ingressImage,
			dispatcherImage:    dispatcherImage,
			rabbitClientSet:    fakerabbitclient.Get(ctx),
			rabbitLister:       rabbitduck.Get(ctx),
			rabbit:             rabbit.New(ctx),
			brokerConfig:       brokerconfig.New(ctx),
		}
		return broker.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetBrokerLister(),
			controller.GetEventRecorder(ctx),
			r, "RabbitMQBroker",
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
func createBrokerSecret(data string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      rabbitBrokerSecretName,
			Labels:    map[string]string{"eventing.knative.dev/broker": "test-broker"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				UID:                brokerUID,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
		},
		StringData: map[string]string{
			"brokerURL": data,
		},
	}

}

func createExchangeSecret(vhost string) *corev1.Secret {
	url := rabbitmqClusterURL
	if vhost != "" {
		url += "/" + vhost
	}
	return createBrokerSecret(url)
}

func createDifferentExchangeSecret() *corev1.Secret {
	return createBrokerSecret("different stuff")
}

// Create Broker Secret that's derived from the rabbitmq cluster status.
func createBrokerSecretFromRabbitmqCluster() *corev1.Secret {
	return createBrokerSecret(rabbitmqClusterURL)
}

// This is the secret that RabbitmqClusters creates.
func createSecretForRabbitmqCluster() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      rabbitmqClusterSecretName,
			Labels:    map[string]string{"eventing.knative.dev/broker": "test-broker"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				UID:                brokerUID,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
		},
		Data: map[string][]byte{
			"password": []byte("mypassword"),
			"username": []byte("myusername"),
		},
	}
}

// This is the secret that RabbitmqClusters creates that's missing username.
func createSecretForRabbitmqClusterNoUser() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      rabbitmqClusterSecretName,
			Labels:    map[string]string{"eventing.knative.dev/broker": "test-broker"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				UID:                brokerUID,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
		},
		Data: map[string][]byte{
			"password": []byte("mypassword"),
		},
	}
}

// This is the secret that RabbitmqClusters creates that's missing password.
func createSecretForRabbitmqClusterNoPassword() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      rabbitmqClusterSecretName,
			Labels:    map[string]string{"eventing.knative.dev/broker": "test-broker"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				UID:                brokerUID,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
		},
		Data: map[string][]byte{
			"username": []byte("myusername"),
		},
	}
}

func createIngressDeployment(opts ...func(*resources.IngressArgs)) *appsv1.Deployment {
	args := &resources.IngressArgs{
		Broker:             &eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: testNS, UID: brokerUID}},
		Image:              ingressImage,
		RabbitMQSecretName: rabbitBrokerSecretName,
		BrokerUrlSecretKey: rabbit.BrokerURLSecretKey,
	}

	for _, o := range opts {
		o(args)
	}
	return resources.MakeIngressDeployment(args)
}

func createDifferentIngressDeployment() *appsv1.Deployment {
	args := &resources.IngressArgs{
		Broker:             &eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: testNS, UID: brokerUID}},
		Image:              "differentImage",
		RabbitMQSecretName: rabbitBrokerSecretName,
		BrokerUrlSecretKey: rabbit.BrokerURLSecretKey,
	}
	return resources.MakeIngressDeployment(args)
}

func WithVhost(args *resources.IngressArgs) {
	args.RabbitMQVhost = rabbitMQVhost
}

func createIngressService() *corev1.Service {
	return resources.MakeIngressService(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: testNS, UID: brokerUID}})
}

func createDifferentIngressService() *corev1.Service {
	svc := resources.MakeIngressService(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: testNS, UID: brokerUID}})
	svc.Spec.Ports = []corev1.ServicePort{{Name: "diff", Port: 9999}}
	return svc
}

func rabbitmqClusterConfigRef() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitMQBrokerName,
		Namespace:  testNS,
		Kind:       "RabbitmqCluster",
		APIVersion: "rabbitmq.com/v1beta1",
	}
}

func rabbitmqClusterConfigRefNoNS() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitMQBrokerName,
		Kind:       "RabbitmqCluster",
		APIVersion: "rabbitmq.com/v1beta1",
	}
}

func rabbitmqBrokerConfigRef() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitMQBrokerConfigName,
		Namespace:  testNS,
		Kind:       "RabbitmqBrokerConfig",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
}

func invalidConfigForRabbitOperator() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       rabbitMQBrokerName,
		Namespace:  testNS,
		Kind:       "NOTRabbitmqCluster",
		APIVersion: "rabbitmq.com/v1beta1",
	}
}

// FilterLabels generates the labels present on all resources representing the filter of the given
// Broker.
func IngressLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerRole": "ingress",
	}
}

func createDispatcherDeployment(opts ...func(*resources.DispatcherArgs)) *appsv1.Deployment {
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerName,
			Namespace: testNS,
			UID:       brokerUID,
		},
		Spec: eventingv1.BrokerSpec{
			Config:   rabbitmqClusterConfigRef(),
			Delivery: delivery,
		},
	}
	args := &resources.DispatcherArgs{
		Broker:             broker,
		Delivery:           delivery,
		Image:              dispatcherImage,
		RabbitMQSecretName: rabbitBrokerSecretName,
		QueueName:          "b.test-namespace.test-broker.dlq.broker-test-uid",
		BrokerUrlSecretKey: "brokerURL",
		BrokerIngressURL:   brokerAddress,
		Subscriber:         DLSAddressable,
		DLX:                true,
	}
	for _, o := range opts {
		o(args)
	}
	return resources.MakeDispatcherDeployment(args)
}

func createExchange(dlx bool, vhost string) *rabbitv1beta1.Exchange {
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerName,
			Namespace: testNS,
			UID:       brokerUID,
		},
	}
	var exchangeName string
	if dlx {
		exchangeName = fmt.Sprintf("b.%s.%s.dlx.%s", testNS, brokerName, brokerUID)
	} else {
		exchangeName = fmt.Sprintf("b.%s.%s.%s", testNS, brokerName, brokerUID)
	}

	return &rabbitv1beta1.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      exchangeName,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(broker),
			},
			Labels: rabbit.Labels(broker, nil, nil),
		},
		Spec: rabbitv1beta1.ExchangeSpec{
			Name:       exchangeName,
			Vhost:      vhost,
			Type:       "headers",
			Durable:    true,
			AutoDelete: false,
			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitMQBrokerName,
				Namespace: testNS,
			},
		},
	}
}

func createReadyExchange(dlx bool, vhost string) *rabbitv1beta1.Exchange {
	e := createExchange(dlx, vhost)
	e.Status = rabbitv1beta1.ExchangeStatus{
		Conditions: []rabbitv1beta1.Condition{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}
	return e
}

func createQueue(dlx bool, kref *duckv1.KReference, vhost string) *rabbitv1beta1.Queue {
	queueType := v1alpha1.ClassicQueueType
	if kref.Kind == "RabbitmqBrokerConfig" {
		queueType = v1alpha1.QuorumQueueType
	}
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerName,
			Namespace: testNS,
			UID:       brokerUID,
		},
	}
	var queueName string
	if dlx {
		queueName = fmt.Sprintf("b.%s.%s.dlq.broker-test-uid", testNS, brokerName)
	} else {
		queueName = fmt.Sprintf("b.%s.%s.broker-test-uid", testNS, brokerName)
	}

	return &rabbitv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      queueName,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(broker),
			},
			Labels: rabbit.Labels(broker, nil, nil),
		},
		Spec: rabbitv1beta1.QueueSpec{
			Name:       queueName,
			Vhost:      vhost,
			Durable:    true,
			AutoDelete: false,
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitMQBrokerName,
				Namespace: testNS,
			},
			Type: string(queueType),
		},
	}
}

func createReadyQueue(dlx bool, kref *duckv1.KReference, vhost string) *rabbitv1beta1.Queue {
	q := createQueue(dlx, kref, vhost)
	q.Status = rabbitv1beta1.QueueStatus{
		Conditions: []rabbitv1beta1.Condition{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}
	return q
}

func createBinding(dlx bool, vhost string) *rabbitv1beta1.Binding {
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerName,
			Namespace: testNS,
			UID:       brokerUID,
		},
	}
	var bindingName string
	if dlx {
		bindingName = fmt.Sprintf("b.%s.%s.dlq.broker-test-uid", testNS, brokerName)
	} else {
		bindingName = fmt.Sprintf("b.%s.%s.broker-test-uid", testNS, brokerName)
	}

	return &rabbitv1beta1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      bindingName,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(broker),
			},
			Labels: rabbit.Labels(broker, nil, nil),
		},
		Spec: rabbitv1beta1.BindingSpec{
			Vhost:           vhost,
			DestinationType: "queue",
			Destination:     bindingName,
			Source:          "b.test-namespace.test-broker.dlx.broker-test-uid",
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitMQBrokerName,
				Namespace: testNS,
			},
			Arguments: getBrokerArguments(),
		},
	}
}

func createReadyBinding(dlx bool, vhost string) *rabbitv1beta1.Binding {
	b := createBinding(dlx, vhost)
	b.Status = rabbitv1beta1.BindingStatus{
		Conditions: []rabbitv1beta1.Condition{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}
	return b
}

func createPolicy(vhost string) *rabbitv1beta1.Policy {
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerName,
			Namespace: testNS,
			UID:       brokerUID,
		},
	}

	policyName := fmt.Sprintf("b.%s.%s.dlq.broker-test-uid", testNS, brokerName)
	dlxName := fmt.Sprintf("b.%s.%s.dlx.%s", testNS, brokerName, brokerUID)

	return &rabbitv1beta1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      policyName,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(broker),
			},
			Labels: rabbit.Labels(broker, nil, nil),
		},
		Spec: rabbitv1beta1.PolicySpec{
			Name:       policyName,
			Priority:   0,
			Vhost:      vhost,
			Pattern:    "broker-test-uid$",
			ApplyTo:    "queues",
			Definition: &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"dead-letter-exchange": %q}`, dlxName))},
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name:      rabbitMQBrokerName,
				Namespace: testNS,
			},
		},
	}
}

func createReadyPolicy(vhost string) *rabbitv1beta1.Policy {
	p := createPolicy(vhost)
	p.Status = rabbitv1beta1.PolicyStatus{
		Conditions: []rabbitv1beta1.Condition{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}
	return p
}

func createRabbitMQBrokerConfig(vhost string) *v1alpha1.RabbitmqBrokerConfig {
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
			Vhost: vhost,
		},
	}
}

func createRabbitMQCluster(vhost string) *unstructured.Unstructured {
	labels := map[string]interface{}{
		eventing.BrokerLabelKey:                 brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
	annotations := map[string]interface{}{
		"eventing.knative.dev/scope": "cluster",
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rabbitmq.com/v1beta1",
			"kind":       "RabbitmqCluster",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              rabbitMQBrokerName,
				"labels":            labels,
				"annotations":       annotations,
			},
			"spec": map[string]interface{}{
				"rabbitmq": map[string]interface{}{
					"additionalConfig": fmt.Sprintf("default_vhost = %s", vhost),
				}},
			"status": map[string]interface{}{
				"defaultUser": map[string]interface{}{
					"secretReference": map[string]interface{}{
						"keys": map[string]interface{}{
							"password": "password",
							"username": "username",
						},
						"name":      rabbitmqClusterSecretName,
						"namespace": testNS,
					},
					"serviceReference": map[string]interface{}{
						"name":      "rabbitmqsvc",
						"namespace": testNS,
					},
				},
				"conditions": []interface{}{
					map[string]interface{}{
						"status": "True",
						"type":   "ReconcileSuccess",
					},
					map[string]interface{}{
						"status": "True",
						"type":   "ClusterAvailable",
					},
				},
			},
		},
	}
}

func createRabbitMQClusterMissingServiceRef() *unstructured.Unstructured {
	labels := map[string]interface{}{
		eventing.BrokerLabelKey:                 brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
	annotations := map[string]interface{}{
		"eventing.knative.dev/scope": "cluster",
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rabbitmq.com/v1beta1",
			"kind":       "RabbitmqCluster",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              rabbitMQBrokerName,
				"labels":            labels,
				"annotations":       annotations,
			},
			"status": map[string]interface{}{
				"defaultUser": map[string]interface{}{
					"secretReference": map[string]interface{}{
						"keys": map[string]interface{}{
							"password": "password",
							"username": "username",
						},
						"name":      rabbitmqClusterSecretName,
						"namespace": testNS,
					},
				},
				"conditions": []interface{}{
					map[string]interface{}{
						"status": "True",
						"type":   "ReconcileSuccess",
					},
					map[string]interface{}{
						"status": "True",
						"type":   "ClusterAvailable",
					},
				},
			},
		},
	}
}

func getBrokerArguments() *runtime.RawExtension {
	arguments := map[string]string{
		"x-match":       "all",
		"x-knative-dlq": brokerName,
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		panic("Failed to marshal json for test, no go.")
	}
	return &runtime.RawExtension{
		Raw: argumentsJson,
	}
}

func AddressableFromDestination(dest *duckv1.Destination) *duckv1.Addressable {
	return &duckv1.Addressable{
		URL:     dest.URI,
		CACerts: dest.CACerts,
	}
}
