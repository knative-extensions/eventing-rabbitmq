//go:build e2e
// +build e2e

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

package e2e

import (
	"os"
	"testing"
	"time"

	"knative.dev/pkg/system"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	// For our e2e testing, we want this linked first so that our
	// system namespace environment variable is defaulted prior to
	// logstream initialization.
	_ "knative.dev/eventing-rabbitmq/test/defaultsystem"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
)

var global environment.GlobalEnvironment

func TestMain(m *testing.M) {
	global = environment.NewStandardGlobalEnvironment()
	os.Exit(m.Run())
}

// TestSmokeBroker makes sure a Broker goes ready as a RabbitMQ Broker Class.
func TestSmokeBroker(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment()
	env.Test(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, SmokeTestBroker())
	env.Finish()
}

// TestSmokeBrokerTrigger makes sure a Broker+Trigger goes ready as a RabbitMQ Broker Class.
func TestSmokeBrokerTrigger(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment()
	env.Test(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, SmokeTestBrokerTrigger())
	env.Finish()
}

// TestBrokerDirect makes sure a Broker can delivery events to a consumer.
func TestBrokerDirect(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, DirectTestBroker())
	env.Finish()
}

// TestVhostBrokerDirect makes sure a Broker can delivery events to a consumer inside a rabbitmq vhost.
func TestVhostBrokerDirect(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQClusterVHost())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, DirectVhostTestBroker())
	env.Finish()
}

// TestBrokerDirect makes sure a Broker can delivery events to a consumer by connecting to a rabbitmq instance via a connection secret
func TestBrokerDirectWithConnectionSecret(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQClusterWithConnectionSecretUri())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, DirectTestBrokerConnectionSecret())
	env.Finish()
}

// TestBrokerDirectSelfSignedCerts makes sure a Broker can delivery events to a consumer while using a RabbitMQ instance with self-signed certificates.
func TestBrokerDirectSelfSignedCerts(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, SetupSelfSignedCerts())
	env.Test(ctx, t, RabbitMQClusterWithTLS())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, DirectTestBroker())
	env.Test(ctx, t, CleanupSelfSignedCerts())
	env.Finish()
}

// TestSourceDirectSelfSignedCerts makes sure a source delivers events to Sink while using a RabbitMQ instance with self-signed certificates.
func TestSourceDirectSelfSignedCerts(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, SetupSelfSignedCerts())
	env.Test(ctx, t, RabbitMQClusterWithTLS())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, DirectSourceTestWithCerts())
	env.Test(ctx, t, CleanupSelfSignedCerts())
	env.Finish()
}

// TestBrokerDLQ makes sure a Broker delivers events to a DLQ.
func TestBrokerDLQ(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, BrokerDLQTest())
	env.Finish()
}

// TestSourceDirect makes sure a source delivers events to Sink.
func TestSourceDirect(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, DirectSourceTest())
	env.Finish()
}

// TestSourceDirect makes sure a source delivers events to Sink by connecting to a rabbitmq instance via a connection secret.
func TestSourceDirectWithConnectionSecret(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQClusterWithConnectionSecretUri())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, DirectSourceConnectionSecretTest())
	env.Finish()
}

func TestSourceVhostSetup(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQClusterVHost())
	env.Test(ctx, t, RecorderFeature())
	env.Test(ctx, t, VHostSourceTest())
	env.Finish()
}

func TestBrokerInDifferentNamespaceThanRabbitMQCluster(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment()
	env.Test(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, NamespacedBrokerTest("broker-namespace"))
	env.Finish()
}

func TestSourceAdapterConcurrency(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, SourceConcurrentReceiveAdapterProcessingTest())
	env.Finish()
}

func TestBrokerDispatcherConcurrency(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	env.Test(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, RecorderFeature(eventshub.ResponseWaitTime(3*time.Second)))
	env.Test(ctx, t, BrokerConcurrentDispatcherTest())
	env.Finish()
}
