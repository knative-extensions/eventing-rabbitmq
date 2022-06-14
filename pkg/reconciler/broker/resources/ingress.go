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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"
)

const (
	ingressContainerName = "ingress"
)

// IngressArgs are the arguments to create a Broker's ingress Deployment.
type IngressArgs struct {
	Broker *eventingv1.Broker
	Image  string
	//ServiceAccountName string
	RabbitMQSecretName string
	BrokerUrlSecretKey string
	Configs            reconcilersource.ConfigAccessor
}

// MakeIngressDeployment creates the in-memory representation of the Broker's ingress Deployment.
func MakeIngressDeployment(args *IngressArgs) *appsv1.Deployment {
	DeploymentName := fmt.Sprintf("%s-broker-ingress", args.Broker.Name)

	envs := []corev1.EnvVar{{
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
		Value: naming.BrokerExchangeName(args.Broker, false),
	}, {
		Name:  "CONTAINER_NAME",
		Value: ingressContainerName,
	}, {
		Name:  "POD_NAME",
		Value: DeploymentName,
	}, {
		Name:  "BROKER_NAME",
		Value: args.Broker.Name,
	}, {
		Name:  "BROKER_NAMESPACE",
		Value: args.Broker.Namespace,
	}}
	if args.Configs != nil {
		envs = append(envs, args.Configs.ToEnvVars()...)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      DeploymentName,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Broker),
			},
			Labels: IngressLabels(args.Broker.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: IngressLabels(args.Broker.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: IngressLabels(args.Broker.Name),
				},
				Spec: corev1.PodSpec{
					//ServiceAccountName: args.ServiceAccountName,
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
						Env: envs,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Name:          "http",
						}},
						// This resource requests and limits comes from performance testing 1500msgs/s with a parallelism of 1000
						// more info in this issue: https://github.com/knative-sandbox/eventing-rabbitmq/issues/703
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("32Mi")},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("400Mi")},
						},
					}},
				},
			},
		},
	}
}

// MakeIngressService creates the in-memory representation of the Broker's ingress Service.
func MakeIngressService(b *eventingv1.Broker) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.Namespace,
			Name:      fmt.Sprintf("%s-broker-ingress", b.Name),
			Labels:    IngressLabels(b.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: IngressLabels(b.Name),
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
// Broker.
func IngressLabels(brokerName string) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey:           brokerName,
		"eventing.knative.dev/brokerRole": "ingress",
	}
}
