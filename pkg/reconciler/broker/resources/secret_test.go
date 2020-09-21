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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"

	"knative.dev/pkg/apis"
	_ "knative.dev/pkg/system/testing"
)

const (
	testRabbitURL = "amqp://localhost.example.com"
)

func TestMakeSecret(t *testing.T) {
	var TrueValue = true
	url, err := apis.ParseURL(testRabbitURL)
	if err != nil {
		t.Errorf("Failed to parse the test URL: %s", err)
	}
	args := &ExchangeArgs{
		Broker:      &eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: ns}},
		RabbitMQURL: url.URL(),
	}

	got := MakeSecret(args)
	want := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      secretName,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
			Labels: map[string]string{
				"eventing.knative.dev/broker": brokerName,
			},
		},
		StringData: map[string]string{
			BrokerURLSecretKey: testRabbitURL,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected diff (-want, +got) = %v", diff)
	}
}
