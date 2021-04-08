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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/system"

	_ "knative.dev/pkg/system/testing"
)

const (
	deploymentName = "testbroker-broker-ingress"
	serviceName    = "testbroker-broker-ingress"
)

func TestMakeIngressDeployment(t *testing.T) {
	var TrueValue = true
	args := &IngressArgs{
		Broker:             &eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: ns}},
		Image:              image,
		RabbitMQSecretName: secretName,
		BrokerUrlSecretKey: brokerURLKey,
	}

	got := MakeIngressDeployment(args)
	want := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      deploymentName,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
			Labels: map[string]string{
				"eventing.knative.dev/broker":     brokerName,
				"eventing.knative.dev/brokerRole": "ingress",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/broker":     brokerName,
					"eventing.knative.dev/brokerRole": "ingress",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"eventing.knative.dev/broker":     brokerName,
						"eventing.knative.dev/brokerRole": "ingress",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: image,
						Name:  "ingress",
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
							Name:  "EXCHANGE_NAME",
							Value: ns + "." + brokerName,
						}},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Name:          "http",
						}},
					}},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected diff (-want, +got) = ", diff)
	}
}

func TestMakeIngressService(t *testing.T) {
	var TrueValue = true
	got := MakeIngressService(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: ns}})
	want := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      serviceName,
			Labels: map[string]string{
				"eventing.knative.dev/broker":     brokerName,
				"eventing.knative.dev/brokerRole": "ingress",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"eventing.knative.dev/broker":     brokerName,
				"eventing.knative.dev/brokerRole": "ingress",
			},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			}, {
				Name: "http-metrics",
				Port: 9090,
			}},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected diff (-want, +got) = ", diff)
	}
}
