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

package rabbit_test

import (
	"testing"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// For our e2e testing, we want this linked first so that our
	// system namespace environment variable is defaulted prior to
	// logstream initialization.
	_ "knative.dev/eventing-rabbitmq/test/defaultsystem"

	"knative.dev/pkg/test/helpers"
	"knative.dev/pkg/test/logstream"
)

// This test is more for debugging the ko publish process.
func TestKoPublish(t *testing.T) {
	ic, err := installer.ProduceImages()
	if err != nil {
		t.Fatalf("failed to produce images, %s", err)
	}

	templateString := `
	rigging.WithImages(map[string]string{
		{{ range $key, $value := . }}"{{ $key }}": "{{ $value }}",{{ end }}
	}),`

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

// TestSmokeBroker makes sure a Broker goes ready as a RabbitMQ Broker Class.
func TestSmokeBroker(t *testing.T) {
	t.Cleanup(logstream.Start(t))
	SmokeTestBrokerImpl(t, helpers.ObjectNameForTest(t))
}

// TestSmokeBrokerTrigger makes sure a Broker+Trigger goes ready as a RabbitMQ Broker Class.
func TestSmokeBrokerTrigger(t *testing.T) {
	t.Cleanup(logstream.Start(t))
	SmokeTestBrokerTriggerImpl(t, helpers.ObjectNameForTest(t), helpers.ObjectNameForTest(t))
}

// TestBrokerDirect makes sure a Broker can delivery events to a consumer.
func TestBrokerDirect(t *testing.T) {
	t.Cleanup(logstream.Start(t))
	DirectTestBrokerImpl(t, helpers.ObjectNameForTest(t))
}
