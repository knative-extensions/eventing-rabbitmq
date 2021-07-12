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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"

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
	Channel *messagingv1beta1.RabbitmqChannel
	Image   string
	//ServiceAccountName string
	RabbitMQHost       string
	RabbitMQSecretName string
	QueueName          string
	BrokerUrlSecretKey string
	ChannelReply       *apis.URL
	Subscriber         *apis.URL
}

func DispatcherName(channelName string) string {
	return fmt.Sprintf("%s-ch-dlq-dispatcher", channelName)
}

// MakeDispatcherDeployment creates the in-memory representation of the Broker's Dispatcher Deployment.
func MakeDispatcherDeployment(args *DispatcherArgs) *appsv1.Deployment {
	one := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Channel.Namespace,
			Name:      DispatcherName(args.Channel.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Channel),
			},
			Labels: DispatcherLabels(args.Channel.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: DispatcherLabels(args.Channel.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: DispatcherLabels(args.Channel.Name),
				},
				Spec: corev1.PodSpec{
					//ServiceAccountName: args.ServiceAccountName,
					Containers: []corev1.Container{{
						Name:  dispatcherContainerName,
						Image: args.Image,
						Env: []corev1.EnvVar{{
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
							Name:  "REPLY_URL",
							Value: args.ChannelReply.String(),
						}},
					}},
				},
			},
		},
	}
}

// DispatcherLabels generates the labels present on all resources representing the dispatcher of the given
// Channel.
func DispatcherLabels(brokerName string) map[string]string {
	return map[string]string{
		"eventing.knative.dev/channelRole": "dispatcher-dlq",
	}
}
