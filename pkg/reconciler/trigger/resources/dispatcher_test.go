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
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/system"

	_ "knative.dev/pkg/system/testing"
)

const (
	brokerName       = "testbroker"
	triggerName      = "testtrigger"
	ns               = "testnamespace"
	image            = "dispatcherimage"
	secretName       = "testbroker-broker-rabbit"
	brokerURLKey     = "testbrokerurl"
	rabbitHost       = "amqp://localhost.example.com"
	queueName        = "testnamespace-testtrigger"
	brokerIngressURL = "http://broker.example.com"
	subscriberURL    = "http://function.example.com"
)

func TestMakeDispatcherDeployment(t *testing.T) {
	var TrueValue = true
	ingressURL := apis.HTTP("broker.example.com")
	sURL := apis.HTTP("function.example.com")
	trigger := &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{Name: triggerName, Namespace: ns},
		Spec:       eventingv1.TriggerSpec{Broker: brokerName},
	}
	args := &DispatcherArgs{
		Trigger:            trigger,
		Image:              image,
		RabbitMQHost:       rabbitHost,
		RabbitMQSecretName: secretName,
		QueueName:          queueName,
		BrokerUrlSecretKey: brokerURLKey,
		BrokerIngressURL:   ingressURL,
		Subscriber:         sURL,
	}

	got := MakeDispatcherDeployment(args)
	one := int32(1)
	want := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Trigger.Namespace,
			Name:      fmt.Sprintf("%s-dispatcher", args.QueueName),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Trigger",
				Name:               triggerName,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
			Labels: map[string]string{
				"eventing.knative.dev/broker":     brokerName,
				"eventing.knative.dev/brokerRole": "dispatcher",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/broker":     brokerName,
					"eventing.knative.dev/brokerRole": "dispatcher",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"eventing.knative.dev/broker":     brokerName,
						"eventing.knative.dev/brokerRole": "dispatcher",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "dispatcher",
						Image: image,
						Env: []corev1.EnvVar{{
							Name:  system.NamespaceEnvKey,
							Value: system.Namespace(),
						}, {
							Name: "BROKER_URL",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretName,
									},
									Key: brokerURLKey,
								},
							},
						}, {
							Name:  "QUEUE_NAME",
							Value: queueName,
						}, {
							Name:  "SUBSCRIBER",
							Value: subscriberURL,
						}, {
							Name:  "BROKER_INGRESS_URL",
							Value: brokerIngressURL,
						}},
					}},
				},
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected diff (-want, +got) = %v", diff)
	}
}
