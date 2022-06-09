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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/pkg/kmeta"
)

const (
	BrokerURLSecretKey = "brokerURL"
)

// MakeSecret creates the secret for Broker deployments for Rabbit Broker.
func MakeSecret(args *ExchangeArgs) *corev1.Secret {
	var name, typeString, ns string
	var owner kmeta.OwnerRefable

	if args.Broker != nil {
		name = args.Broker.Name
		owner = args.Broker
		typeString = "broker"
		ns = args.Broker.Namespace
	} else if args.Source != nil {
		owner = args.Source
		name = args.Source.Name
		typeString = "source"
		ns = args.Source.Namespace
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      SecretName(name, typeString),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(owner),
			},
			Labels: SecretLabels(name, typeString),
		},
		StringData: map[string]string{
			BrokerURLSecretKey: args.RabbitMQURL.String(),
		},
	}
}

func SecretName(resourceName, typeString string) string {
	return fmt.Sprintf("%s-%s-rabbit", resourceName, typeString)
}

// SecretLabels generates the labels present on all resources representing the
// secret of the given Broker.
func SecretLabels(resourceName, typeString string) map[string]string {
	var label string
	if typeString == "broker" {
		label = eventing.BrokerLabelKey
	} else if typeString == "source" {
		label = SourceLabelKey
	}
	return map[string]string{
		label: resourceName,
	}
}

// reconcileSecret reconciles the K8s Secret 's'.
func ReconcileSecret(ctx context.Context, secretLister corev1listers.SecretLister, kubeClientSet kubernetes.Interface, s *corev1.Secret) error {
	current, err := secretLister.Secrets(s.Namespace).Get(s.Name)
	if apierrs.IsNotFound(err) {
		_, err = kubeClientSet.CoreV1().Secrets(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !equality.Semantic.DeepDerivative(s.StringData, current.StringData) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.StringData = s.StringData
		_, err = kubeClientSet.CoreV1().Secrets(desired.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
