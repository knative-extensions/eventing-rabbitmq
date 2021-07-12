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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"
)

const (
	ingressContainerName = "ingress"
)

// IngressArgs are the arguments to create a Broker's ingress Deployment.
type IngressArgs struct {
	Channel            *messagingv1beta1.RabbitmqChannel
	Image              string
	RabbitMQSecretName string
	BrokerUrlSecretKey string
}

// MakeIngress creates the in-memory representation of the Broker's ingress Deployment.
func MakeIngressDeployment(args *IngressArgs) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Channel.Namespace,
			Name:      fmt.Sprintf("%s-channel-ingress", args.Channel.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Channel),
			},
			Labels: IngressLabels(args.Channel.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: IngressLabels(args.Channel.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: IngressLabels(args.Channel.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: args.Image,
						Name:  ingressContainerName,
						// LivenessProbe: &corev1.Probe{
						// 	Handler: corev1.Handler{
						// 		HTTPGet: &corev1.HTTPGetAction{
						// 			Path: "/healthz",
						// 			Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						// 		},
						// 	},
						// 	InitialDelaySeconds: 5,
						// 	PeriodSeconds:       2,
						// },
						Env: []corev1.EnvVar{{
							Name:  system.NamespaceEnvKey,
							Value: system.Namespace(),
						}, {
							Name: "BROKER_URL",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: args.RabbitMQSecretName,
									},
									Key: args.BrokerUrlSecretKey,
								},
							},
						}, {
							Name:  "EXCHANGE_NAME",
							Value: naming.ChannelExchangeName(args.Channel, false),
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
}

// MakeIngressService creates the in-memory representation of the Broker's ingress Service.
func MakeIngressService(c *messagingv1beta1.RabbitmqChannel) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.Namespace,
			Name:      fmt.Sprintf("%s-channel-ingress", c.Name),
			Labels:    IngressLabels(c.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(c),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: IngressLabels(c.Name),
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
}

// IngressLabels generates the labels present on all resources representing the ingress of the given
// Channel.
func IngressLabels(channelName string) map[string]string {
	return map[string]string{
		"eventing.knative.dev/channelRole": "ingress",
	}
}
