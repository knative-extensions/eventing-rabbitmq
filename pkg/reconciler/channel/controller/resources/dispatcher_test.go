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
	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/system"

	_ "knative.dev/pkg/system/testing"
)

const (
	ns              = "testnamespace"
	image           = "dispatcherimage"
	secretName      = "test-channel-channel-rabbit"
	brokerURLKey    = "testbrokerurl"
	rabbitHost      = "amqp://localhost.example.com"
	queueName       = "testnamespace-testtrigger"
	channelReplyURL = "http://myreply.example.com"
	subscriberURL   = "http://function.example.com"
)

func TestMakeDispatcherDeployment(t *testing.T) {
	var TrueValue = true
	sURL := apis.HTTP("function.example.com")
	bURL := apis.HTTP("myreply.example.com")
	channel := &messagingv1beta1.RabbitmqChannel{
		ObjectMeta: metav1.ObjectMeta{Name: channelName, Namespace: ns, UID: channelUID},
	}
	args := &DispatcherArgs{
		Channel:            channel,
		Image:              image,
		RabbitMQHost:       rabbitHost,
		RabbitMQSecretName: secretName,
		QueueName:          queueName,
		BrokerUrlSecretKey: brokerURLKey,
		Subscriber:         sURL,
		ChannelReply:       bURL,
	}

	got := MakeDispatcherDeployment(args)
	one := int32(1)
	want := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Channel.Namespace,
			Name:      "test-channel-ch-dlq-dispatcher",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "messaging.knative.dev/v1beta1",
				Kind:               "RabbitmqChannel",
				Name:               channelName,
				UID:                channelUID,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
			Labels: map[string]string{
				"eventing.knative.dev/channelRole": "dispatcher-dlq",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/channelRole": "dispatcher-dlq",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"eventing.knative.dev/channelRole": "dispatcher-dlq",
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
							Name: "RABBIT_URL",
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
							Name:  "REQUEUE",
							Value: "false",
						}, {
							Name:  "REPLY_URL",
							Value: channelReplyURL,
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
