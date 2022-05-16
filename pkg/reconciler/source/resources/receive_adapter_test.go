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
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1alpha12 "knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

func TestMakeReceiveAdapter(t *testing.T) {
	var retry int32 = 5
	parallelism := 10
	backoffDelay := "PT0.1S"

	for _, tt := range []struct {
		name          string
		backoffPolicy eventingduckv1.BackoffPolicyType
	}{{
		name:          "Backoff policy linear",
		backoffPolicy: eventingduckv1.BackoffPolicyLinear,
	},
		{
			name:          "Backoff policy exponential",
			backoffPolicy: eventingduckv1.BackoffPolicyExponential,
		}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			src := &v1alpha12.RabbitmqSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-name",
					Namespace: "source-namespace",
				},
				Spec: v1alpha12.RabbitmqSourceSpec{
					ServiceAccountName: "source-svc-acct",
					Broker:             "amqp://guest:guest@localhost:5672/",
					User: v1alpha12.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "the-user-secret",
							},
							Key: "user",
						},
					},
					Password: v1alpha12.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "the-password-secret",
							},
							Key: "password",
						},
					},
					Predeclared: true,
					ExchangeConfig: v1alpha12.RabbitmqSourceExchangeConfigSpec{
						Name:       "logs",
						Type:       "topic",
						Durable:    true,
						AutoDelete: false,
					},
					QueueConfig: v1alpha12.RabbitmqSourceQueueConfigSpec{
						Name:       "",
						RoutingKey: "*.critical",
						AutoDelete: false,
					},
					ChannelConfig: v1alpha12.RabbitmqChannelConfigSpec{
						Parallelism: &parallelism,
					},
					Retry:         &retry,
					BackoffDelay:  &backoffDelay,
					BackoffPolicy: &tt.backoffPolicy,
				},
			}

			got := MakeReceiveAdapter(&ReceiveAdapterArgs{
				Image:  "test-image",
				Source: src,
				Labels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
				SinkURI: "sink-uri",
			})

			boPolicy := tt.backoffPolicy
			one := int32(1)
			if boPolicy == "" {
				boPolicy = eventingduckv1.BackoffPolicyExponential
			}

			want := &v1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:         "rabbitmqsource-source-name-",
					Namespace:    "source-namespace",
					GenerateName: "source-name-",
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "sources.knative.dev/v1alpha1",
							Kind:               "RabbitmqSource",
							Name:               "source-name",
							Controller:         &[]bool{true}[0],
							BlockOwnerDeletion: &[]bool{true}[0],
						},
					},
				},
				Spec: v1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-key1": "test-value1",
							"test-key2": "test-value2",
						},
					},
					Replicas: &one,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"sidecar.istio.io/inject": "true",
							},
							Labels: map[string]string{
								"test-key1": "test-value1",
								"test-key2": "test-value2",
							},
						},
						Spec: corev1.PodSpec{
							ServiceAccountName: "source-svc-acct",
							Containers: []corev1.Container{
								{
									Name:            "receive-adapter",
									Image:           "test-image",
									ImagePullPolicy: "IfNotPresent",
									Env: []corev1.EnvVar{
										{
											Name:  "RABBITMQ_BROKERS",
											Value: "amqp://guest:guest@localhost:5672/",
										},
										{
											Name: "RABBITMQ_USER",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "the-user-secret",
													},
													Key: "user",
												},
											},
										},
										{
											Name: "RABBITMQ_PASSWORD",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "the-password-secret",
													},
													Key: "password",
												},
											},
										},
										{
											Name:  "RABBITMQ_ROUTING_KEY",
											Value: "*.critical",
										},
										{
											Name:  "RABBITMQ_CHANNEL_CONFIG_QOS_GLOBAL",
											Value: "false",
										},
										{
											Name:  "RABBITMQ_CHANNEL_CONFIG_PARALLELISM",
											Value: "10",
										},
										{
											Name:  "RABBITMQ_EXCHANGE_CONFIG_NAME",
											Value: "logs",
										},
										{
											Name:  "RABBITMQ_EXCHANGE_CONFIG_TYPE",
											Value: "topic",
										},
										{
											Name:  "RABBITMQ_EXCHANGE_CONFIG_DURABLE",
											Value: "true",
										},
										{
											Name:  "RABBITMQ_EXCHANGE_CONFIG_AUTO_DELETE",
											Value: "false",
										},
										{
											Name:  "RABBITMQ_QUEUE_CONFIG_NAME",
											Value: "",
										},
										{
											Name:  "RABBITMQ_QUEUE_CONFIG_AUTO_DELETE",
											Value: "false",
										},
										{
											Name:  "RABBITMQ_PREDECLARED",
											Value: "true",
										},
										{
											Name:  "SINK_URI",
											Value: "sink-uri",
										},
										{
											Name:  "K_SINK",
											Value: "sink-uri",
										},
										{
											Name:  "NAME",
											Value: "source-name",
										},
										{
											Name:  "NAMESPACE",
											Value: "source-namespace",
										},
										{
											Name: "K_LOGGING_CONFIG",
										},
										{
											Name: "K_METRICS_CONFIG",
										},
										{
											Name: "RABBITMQ_VHOST",
										},
										{
											Name:  "HTTP_SENDER_RETRY",
											Value: "5",
										},
										{
											Name:  "HTTP_SENDER_BACKOFF_POLICY",
											Value: string(boPolicy),
										},
										{
											Name:  "HTTP_SENDER_BACKOFF_DELAY",
											Value: "100ms",
										},
									},
								},
							},
						},
					},
				},
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("unexpected deploy (-want, +got) = %v", diff)
			}
		})
	}
}
