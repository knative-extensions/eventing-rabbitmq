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

	"github.com/rickb777/date/period"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"
)

const (
	dispatcherContainerName = "dispatcher"
)

// DispatcherArgs are the arguments to create a Broker's Dispatcher Deployment that handles
// DeadLetterSink deliveries.
type DispatcherArgs struct {
	Delivery *eventingduckv1.DeliverySpec
	Broker   *eventingv1.Broker
	Image    string
	//ServiceAccountName string
	RabbitMQHost       string
	RabbitMQSecretName string
	QueueName          string
	BrokerUrlSecretKey string
	BrokerIngressURL   *apis.URL
	Subscriber         *apis.URL
	Configs            reconcilersource.ConfigAccessor
}

func DispatcherName(brokerName string) string {
	return fmt.Sprintf("%s-dlq-dispatcher", brokerName)
}

// MakeDispatcherDeployment creates the in-memory representation of the Broker's Dispatcher Deployment.
func MakeDispatcherDeployment(args *DispatcherArgs) *appsv1.Deployment {
	one := int32(1)
	envs := []corev1.EnvVar{{
		Name:  system.NamespaceEnvKey,
		Value: system.Namespace(),
	}, {
		Name: "RABBIT_URL",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: args.RabbitMQSecretName,
				},
				Key: args.BrokerUrlSecretKey,
			},
		},
	}, {
		Name:  "QUEUE_NAME",
		Value: args.QueueName,
	}, {
		Name:  "SUBSCRIBER",
		Value: args.Subscriber.String(),
	}, {
		// Do not requeue failed events
		Name:  "REQUEUE",
		Value: "false",
	}, {
		Name:  "BROKER_INGRESS_URL",
		Value: args.BrokerIngressURL.String(),
	}}
	if args.Configs != nil {
		envs = append(envs, args.Configs.ToEnvVars()...)
	}
	if args.Delivery != nil {
		if args.Delivery.Retry != nil {
			envs = append(envs,
				corev1.EnvVar{
					Name:  "RETRY",
					Value: fmt.Sprint(*args.Delivery.Retry),
				})

		} else {
			envs = append(envs,
				corev1.EnvVar{
					Name:  "RETRY",
					Value: "5",
				})
		}
		if args.Delivery.BackoffPolicy != nil {
			envs = append(envs,
				corev1.EnvVar{
					Name:  "BACKOFF_POLICY",
					Value: string(*args.Delivery.BackoffPolicy),
				})
		}
		if args.Delivery.BackoffDelay != nil {
			p, _ := period.Parse(*args.Delivery.BackoffDelay)
			envs = append(envs,
				corev1.EnvVar{
					Name:  "BACKOFF_DELAY",
					Value: p.DurationApprox().String(),
				})
		}
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      DispatcherName(args.Broker.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Broker),
			},
			Labels: DispatcherLabels(args.Broker.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: DispatcherLabels(args.Broker.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: DispatcherLabels(args.Broker.Name),
				},
				Spec: corev1.PodSpec{
					//ServiceAccountName: args.ServiceAccountName,
					Containers: []corev1.Container{{
						Name:  dispatcherContainerName,
						Image: args.Image,
						Env:   envs,
						// This resource requests and limits comes from performance testing 1500msgs/s with a parallelism of 1000
						// more info in this issue: https://github.com/knative-sandbox/eventing-rabbitmq/issues/703
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("300m"),
								corev1.ResourceMemory: resource.MustParse("64Mi")},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4000m"),
								corev1.ResourceMemory: resource.MustParse("600Mi")},
						},
					}},
				},
			},
		},
	}
}

// DispatcherLabels generates the labels present on all resources representing the dispatcher of the given
// Broker.
func DispatcherLabels(brokerName string) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey:           brokerName,
		"eventing.knative.dev/brokerRole": "dispatcher-dlq",
	}
}
