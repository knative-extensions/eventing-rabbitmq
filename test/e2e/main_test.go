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
	"flag"
	"fmt"
	"os"
	"testing"
	"text/template"

	"knative.dev/eventing/test/rekt/features"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	"knative.dev/pkg/injection"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	// For our e2e testing, we want this linked first so that our
	// system namespace environment variable is defaulted prior to
	// logstream initialization.
	_ "knative.dev/eventing-rabbitmq/test/defaultsystem"
	"knative.dev/reconciler-test/pkg/environment"
)

func init() {
	environment.InitFlags(flag.CommandLine)
}

var global environment.GlobalEnvironment

// This test is more for debugging the ko publish process.
func TestKoPublish(t *testing.T) {
	ic, err := environment.ProduceImages()
	if err != nil {
		panic(fmt.Errorf("failed to produce images, %s", err))
	}

	templateString := `
// The following could be used to bypass the image generation process.
import "knative.dev/reconciler-test/pkg/environment"
func init() {
	environment.WithImages(map[string]string{
		{{ range $key, $value := . }}"{{ $key }}": "{{ $value }}",
		{{ end }}
	})
}
`

	tp := template.New("t")
	temp, err := tp.Parse(templateString)
	if err != nil {
		panic(err)
	}

	err = temp.Execute(os.Stdout, ic)
	if err != nil {
		panic(err)
	}
	_, _ = fmt.Fprint(os.Stdout, "\n\n")
}

func TestMain(m *testing.M) {
	flag.Parse()
	ctx, startInformers := injection.EnableInjectionOrDie(nil, nil) //nolint
	startInformers()
	global = environment.NewGlobalEnvironment(ctx)
	os.Exit(m.Run())
}

// TestSmokeBroker makes sure a Broker goes ready as a RabbitMQ Broker Class.
func TestSmokeBroker(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment()
	env.Prerequisite(ctx, t, RabbitMQCluster())
	brokerName := feature.MakeRandomK8sName("broker")
	env.Test(ctx, t, SmokeTestBroker(brokerName))
	env.Finish()
}

// TestSmokeBrokerTrigger makes sure a Broker+Trigger goes ready as a RabbitMQ Broker Class.
func TestSmokeBrokerTrigger(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment()
	env.Prerequisite(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, SmokeTestBrokerTrigger())
	env.Finish()
}

// TestBrokerDirect makes sure a Broker can delivery events to a consumer.
func TestBrokerDirect(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment()
	env.Prerequisite(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, DirectTestBroker())
	env.Finish()
}

// TestBrokerDLQ makes sure a Broker delivers events to a DLQ.
func TestBrokerDLQ(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment()
	env.Prerequisite(ctx, t, RabbitMQCluster())
	env.Test(ctx, t, BrokerDLQTest())
	env.Finish()
}

// TestBrokerAsMiddleware makes sure a Broker acts as middleware.
func TestBrokerAsMiddleware(t *testing.T) {
	t.Parallel()
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	)
	brokerName := feature.MakeRandomK8sName("broker")
	env.Prerequisite(ctx, t, RabbitMQCluster())
	env.Prerequisite(ctx, t, SmokeTestBroker(brokerName))
	env.Test(ctx, t, features.BrokerAsMiddleware(brokerName))
	env.Finish()
}
