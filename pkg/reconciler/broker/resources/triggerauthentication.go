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

	kedav1alpha1 "knative.dev/eventing-rabbitmq/broker/pkg/internal/thirdparty/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/kmeta"
)

type TriggerAuthenticationArgs struct {
	Broker     *eventingv1beta1.Broker
	SecretName string
	SecretKey  string
}

func MakeTriggerAuthentication(args *TriggerAuthenticationArgs) *kedav1alpha1.TriggerAuthentication {
	return &kedav1alpha1.TriggerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      fmt.Sprintf("%s-trigger-auth", args.Broker.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Broker),
			},
			Labels: IngressLabels(args.Broker.Name),
		},
		Spec: kedav1alpha1.TriggerAuthenticationSpec{
			SecretTargetRef: []kedav1alpha1.AuthSecretTargetRef{
				{
					Parameter: "host",
					Name:      args.SecretName,
					Key:       args.SecretKey,
				},
			},
			Env: []kedav1alpha1.AuthEnvironment{},
		},
	}
}
