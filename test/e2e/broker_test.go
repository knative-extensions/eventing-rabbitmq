//go:build e2e
// +build e2e

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
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	brokere2e "knative.dev/eventing-rabbitmq/test/e2e/config/broker"
	"knative.dev/eventing-rabbitmq/test/e2e/config/brokersecret"
	"knative.dev/eventing-rabbitmq/test/e2e/config/brokertrigger"
	"knative.dev/eventing-rabbitmq/test/e2e/config/brokertriggervhost"
	"knative.dev/eventing-rabbitmq/test/e2e/config/dlq"
	smokebrokere2e "knative.dev/eventing-rabbitmq/test/e2e/config/smoke/broker"
	smokebrokertriggere2e "knative.dev/eventing-rabbitmq/test/e2e/config/smoke/brokertrigger"
	brokerresources "knative.dev/eventing/test/rekt/resources/broker"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	_ "knative.dev/pkg/system/testing"
)

//
// producer ---> broker --[trigger]--> recorder
//

// DirectTestBrokerImpl makes sure an RabbitMQ Broker delivers events to a single consumer.
func DirectTestBroker() *feature.Feature {
	f := new(feature.Feature)
	prober := eventshub.NewProber()
	prober.SetTargetResource(brokerresources.GVR(), "testbroker")
	prober.SenderFullEvents(5)

	f.Setup("install test resources", brokertrigger.Install(brokertrigger.Topology{
		Triggers: []duckv1.KReference{
			{
				Kind: "Service",
				Name: "recorder",
			},
		},
	}))
	f.Setup("RabbitMQ broker goes ready", AllGoReady)
	f.Setup("install source", prober.SenderInstall("source"))
	f.Requirement("sender is finished", prober.SenderDone("source"))

	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(ctx, t, 5)
			})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

const ingressRestartEventCount = 60

// start sending events and od restarts, all events must be sent and all must
// be received. see #1669
func BrokerIngressRollingRestartNoEventLoss() *feature.Feature {
	f := new(feature.Feature)
	prober := eventshub.NewProber()
	prober.SetTargetResource(brokerresources.GVR(), "testbroker")
	prober.SenderFullEvents(ingressRestartEventCount)

	f.Setup("install test resources", brokertrigger.Install(brokertrigger.Topology{
		Triggers: []duckv1.KReference{
			{
				Kind: "Service",
				Name: "recorder",
			},
		},
	}))
	f.Setup("RabbitMQ broker goes ready", AllGoReady)
	f.Requirement("start streaming events", prober.SenderInstall("source"))

	const minSentBeforeRestart = 5
	f.Requirement("rolling-restart the ingress mid-stream", rolloutRestartAndWait("testbroker-broker-ingress", prober, "source", minSentBeforeRestart))

	f.Assert("ingress accepted every event during the restart", prober.AssertSentAll("source"))

	f.Assert("recorder received every event", func(ctx context.Context, t feature.T) {
		eventshub.StoreFromContext(ctx, "recorder").AssertExact(ctx, t, ingressRestartEventCount)
	})

	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

func rolloutRestartAndWait(deploymentName string, prober *eventshub.EventProber, senderPrefix string, minSent int) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		ns := environment.FromContext(ctx).Namespace()
		deployments := kubeclient.Get(ctx).AppsV1().Deployments(ns)

		if err := wait.PollUntilContextTimeout(ctx, time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := deployments.Get(ctx, deploymentName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return err == nil, err
		}); err != nil {
			t.Fatalf("ingress deployment %s never appeared: %v", deploymentName, err)
		}

		// wait until the sender is actually streaming so the rollout overlaps in-flight traffic.
		if err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 2*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			defer func() {
				if r := recover(); r != nil {
					done = false
				}
			}()
			return len(prober.SentBy(ctx, senderPrefix)) >= minSent, nil
		}); err != nil {
			t.Fatalf("sender %q did not reach %d sent events before rollout: %v", senderPrefix, minSent, err)
		}

		// Trigger a rollout the same way `kubectl rollout restart` does.
		patch := []byte(fmt.Sprintf(
			`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
			time.Now().Format(time.RFC3339)))
		updated, err := deployments.Patch(ctx, deploymentName, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			t.Fatalf("failed to trigger rollout restart of %s: %v", deploymentName, err)
		}
		targetGen := updated.Generation

		if err := wait.PollUntilContextTimeout(ctx, time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
			d, err := deployments.Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			desired := int32(1)
			if d.Spec.Replicas != nil {
				desired = *d.Spec.Replicas
			}
			return d.Status.ObservedGeneration >= targetGen &&
				d.Status.UpdatedReplicas == desired &&
				d.Status.Replicas == desired &&
				d.Status.AvailableReplicas == desired, nil
		}); err != nil {
			t.Fatalf("ingress rollout did not complete: %v", err)
		}
	}
}

func DirectTestBrokerConnectionSecret() *feature.Feature {
	f := new(feature.Feature)
	prober := eventshub.NewProber()
	prober.SetTargetResource(brokerresources.GVR(), "testbroker")
	prober.SenderFullEvents(5)

	f.Setup("install test resources", brokersecret.Install(brokersecret.Topology{
		Triggers: []duckv1.KReference{
			{
				Kind: "Service",
				Name: "recorder",
			},
		},
	}))
	f.Setup("RabbitMQ broker goes ready", AllGoReady)
	f.Setup("install source", prober.SenderInstall("source"))

	f.Requirement("sender is finished", prober.SenderDone("source"))

	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(ctx, t, 5)
			})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

//
// producer ---> rabbitmq --[vhost(broker)]--> trigger --> recorder
//

// DirectVhostTestBroker makes sure an RabbitMQ Broker is created on the desired vhost.
func DirectVhostTestBroker() *feature.Feature {
	f := new(feature.Feature)
	prober := eventshub.NewProber()
	prober.SetTargetResource(brokerresources.GVR(), "testbroker")
	prober.SenderFullEvents(5)

	f.Setup("install test resources", brokertriggervhost.Install(brokertriggervhost.Topology{
		Triggers: []duckv1.KReference{
			{
				Kind: "Service",
				Name: "recorder",
			},
		},
	}))
	f.Setup("RabbitMQ broker goes ready", AllGoReady)
	f.Setup("install source", prober.SenderInstall("source"))

	f.Requirement("sender is finished", prober.SenderDone("source"))

	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(ctx, t, 5)
			})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

// BrokerConcurrentDispatcherTest verifies that the dispatcher sends events
// concurrently. It does this by sending 2 events to an event recorder that
// takes 1 second to respond to each event. It waits for both events to be
// received and then calculates the time between events to determine that the
// dispatcher did not block on receiving a response to the first event before
// dispatching the second.
func BrokerConcurrentDispatcherTest() *feature.Feature {
	f := new(feature.Feature)
	prober := eventshub.NewProber()
	prober.SetTargetResource(brokerresources.GVR(), "testbroker")
	prober.SenderFullEvents(2)

	f.Setup("install test resources", brokertrigger.Install(brokertrigger.Topology{
		Parallelism: 10,
		Triggers: []duckv1.KReference{{
			Kind: "Service",
			Name: "recorder",
		}},
	}))
	f.Setup("RabbitMQ broker goes ready", AllGoReady)
	f.Setup("install source", prober.SenderInstall("source"))

	f.Requirement("sender is finished", prober.SenderDone("source"))

	f.Assert("the dispatcher sends events concurrently", func(ctx context.Context, t feature.T) {
		events := eventshub.StoreFromContext(ctx, "recorder").AssertExact(ctx, t, 2)
		diff := events[1].Time.Sub(events[0].Time)
		if diff >= 3*time.Second {
			t.Fatalf("expected dispatch to happen concurrently but were sequential. time elapsed between events: %v", diff)
		}
	})
	f.Teardown("Delete feature resources", f.DeleteResources)

	return f
}

//
// producer ---> broker --[trigger]--> failer (always fails)
//                  |
//                  +--[DLQ]--> recorder
//

// BrokerDLQTestImpl makes sure an RabbitMQ Broker delivers events to a DLQ.
func BrokerDLQTest() *feature.Feature {
	f := new(feature.Feature)
	prober := eventshub.NewProber()
	prober.SetTargetResource(brokerresources.GVR(), "testbroker")
	prober.SenderFullEvents(5)

	f.Setup("dlq works", dlq.Install())
	f.Setup("RabbitMQ broker goes ready", AllGoReady)
	f.Setup("install source", prober.SenderInstall("source"))

	f.Requirement("sender is finished", prober.SenderDone("source"))

	f.Alpha("RabbitMQ source").
		Must("the recorder received all sent events within the time",
			func(ctx context.Context, t feature.T) {
				// TODO: Use constraint matching instead of just counting number of events.
				eventshub.StoreFromContext(ctx, "recorder").AssertAtLeast(ctx, t, 5)
			})
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

// MultipleBrokersUsingSingleRabbitMQClusterTest checks that Brokers in different namespaces can use the same RabbitMQ cluster
func NamespacedBrokerTest(namespace string) *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install Broker in different namespace", brokere2e.Install(namespace))
	f.Requirement("Broker and all dependencies - Exchange, Queue & Binding - be ready", AllGoReady)
	f.Teardown("delete Broker namespace and all objects", brokere2e.Uninstall(namespace))
	return f
}

// SmokeTestBrokerImpl makes sure an RabbitMQ Broker goes ready.
func SmokeTestBrokerTrigger() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install a broker", smokebrokertriggere2e.Install())
	f.Requirement("RabbitMQ broker goes ready", AllGoReady)
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

// SmokeTestBrokerImpl makes sure an RabbitMQ Broker goes ready.
func SmokeTestBroker() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install a broker", smokebrokere2e.Install())
	f.Requirement("RabbitMQ broker goes ready", AllGoReady)
	f.Teardown("Delete feature resources", f.DeleteResources)
	return f
}

func AllGoReady(ctx context.Context, t feature.T) {
	env := environment.FromContext(ctx)
	for _, ref := range env.References() {
		if !strings.Contains(ref.APIVersion, "knative.dev") || ref.Kind == "RabbitmqBrokerConfig" {
			// Let's not care so much about checking the status of non-Knative
			// resources.
			// RabbitmqBrokerConfig isn't a reconciled resources, so won't have any Status
			continue
		}
		if err := k8s.WaitForReadyOrDone(ctx, t, ref, interval, timeout); err != nil {
			t.Fatal("failed to wait for ready or done, ", err)
		}
	}
	t.Log("all resources ready")
}
