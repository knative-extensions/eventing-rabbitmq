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
	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"

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
		Channel:     &messagingv1beta1.RabbitmqChannel{ObjectMeta: metav1.ObjectMeta{Name: channelName, Namespace: ns, UID: channelUID}},
		RabbitMQURL: url.URL(),
	}

	got := MakeSecret(args)
	want := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      secretName,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "messaging.knative.dev/v1beta1",
				Kind:               "RabbitmqChannel",
				Name:               channelName,
				UID:                channelUID,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
			Labels: map[string]string{
				"messaging.knative.dev/channel": channelName,
			},
		},
		StringData: map[string]string{
			BrokerURLSecretKey: testRabbitURL,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected diff (-want, +got) = ", diff)
	}
}
