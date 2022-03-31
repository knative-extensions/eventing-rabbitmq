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
	"fmt"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/pkg/kmeta"
)

const (
	BrokerURLSecretKey = "brokerURL"
)

// MakeSecret creates the secret for Broker deployments for Rabbit Broker.
func MakeSecret(args *rabbit.ExchangeArgs) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      SecretName(args.Broker.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Broker),
			},
			Labels: SecretLabels(args.Broker.Name),
		},
		StringData: map[string]string{
			BrokerURLSecretKey: args.RabbitMQURL.String(),
		},
	}
}

func SecretName(brokerName string) string {
	return fmt.Sprintf("%s-broker-rabbit", brokerName)
}

// SecretLabels generates the labels present on all resources representing the
// secret of the given Broker.
func SecretLabels(brokerName string) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey: brokerName,
	}
}
