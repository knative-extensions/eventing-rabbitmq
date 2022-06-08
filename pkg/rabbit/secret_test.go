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

package rabbit

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"

	"knative.dev/pkg/apis"
	_ "knative.dev/pkg/system/testing"
)

const (
	testRabbitURL    = "amqp://localhost.example.com"
	brokerName       = "test-broker"
	ns               = "testnamespace"
	brokerSecretName = "test-broker-broker-rabbit"
	sourceSecretName = "test-source-source-rabbit"
)

func TestMakeSecret(t *testing.T) {
	var TrueValue = true
	url, err := apis.ParseURL(testRabbitURL)
	if err != nil {
		t.Errorf("Failed to parse the test URL: %s", err)
	}

	for _, tt := range []struct {
		name string
		args *ExchangeArgs
		want *corev1.Secret
	}{
		{
			name: "test broker secret name",
			args: &ExchangeArgs{
				Broker:      &eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: ns}},
				RabbitMQURL: url.URL(),
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
					Name:      brokerSecretName,
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
			},
		}, {
			name: "test source secret name",
			args: &ExchangeArgs{
				Source:      &v1alpha1.RabbitmqSource{ObjectMeta: metav1.ObjectMeta{Name: sourceName, Namespace: ns}},
				RabbitMQURL: url.URL(),
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
					Name:      sourceSecretName,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "sources.knative.dev/v1alpha1",
						Kind:               "RabbitmqSource",
						Name:               sourceName,
						Controller:         &TrueValue,
						BlockOwnerDeletion: &TrueValue,
					}},
					Labels: map[string]string{
						SourceLabelKey: sourceName,
					},
				},
				StringData: map[string]string{
					BrokerURLSecretKey: testRabbitURL,
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeSecret(tt.args)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Error("unexpected diff (-want, +got) = ", diff)
			}
		})
	}
}
