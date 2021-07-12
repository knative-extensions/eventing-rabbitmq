/*
Copyright 2021 The Knative Authors

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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
)

const (
	BrokerURLSecretKey = "brokerURL"
)

// MakeSecret creates the secret for Channel deployments for Rabbit Broker.
func MakeSecret(args *ExchangeArgs) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Channel.Namespace,
			Name:      SecretName(args.Channel.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Channel),
			},
			Labels: SecretLabels(args.Channel.Name),
		},
		StringData: map[string]string{
			BrokerURLSecretKey: args.RabbitMQURL.String(),
		},
	}
}

func SecretName(channelName string) string {
	return fmt.Sprintf("%s-channel-rabbit", channelName)
}

// SecretLabels generates the labels present on all resources representing the
// secret of the given Channel.
func SecretLabels(channelName string) map[string]string {
	return map[string]string{
		"messaging.knative.dev/channel": channelName,
	}
}
