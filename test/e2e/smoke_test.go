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
	"strings"
	"testing"
	"time"

	"github.com/n3wscott/rigging"
)

// SmokeTestBrokerImpl makes sure an RabbitMQ Broker goes ready.
func SmokeTestBrokerImpl(t *testing.T, brokerName string) {

	opts := []rigging.Option{}

	rig, err := rigging.NewInstall(opts, []string{"rabbitmq", "smoke/broker"}, map[string]string{
		"brokerName": brokerName,
	})
	if err != nil {
		t.Fatalf("failed to create rig, %s", err)
	}
	t.Logf("Created a new testing rig at namespace %s.", rig.Namespace())

	// Uninstall deferred.
	defer func() {
		if err := rig.Uninstall(); err != nil {
			t.Errorf("failed to uninstall, %s", err)
		}
	}()

	refs := rig.Objects()
	for _, r := range refs {
		if !strings.Contains(r.APIVersion, "knative.dev") {
			// Let's not care so much about checking the status of non-knative
			// resources.
			continue
		}
		_, err := rig.WaitForReadyOrDone(r, 5*time.Minute)
		if err != nil {
			t.Fatalf("failed to wait for ready or done, %s", err)
		}
		// Pass!
	}
}

/*
Smoke test does not work because the secret needs to be in the form (from the readme):

```sh
PASSWORD=$(kubectl get secret --namespace default rokn-rabbitmq -o jsonpath="{.data.rabbitmq-password}" | base64 --decode)

kubectl create secret generic rokn-rabbitmq-broker-secret \
    --from-literal=brokerURL="amqp://user:$PASSWORD@rokn-rabbitmq.default:5672"
```

*/
