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
	"strconv"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/ptr"
)

type ReceiveAdapterArgs struct {
	Image                string
	Source               *v1alpha1.RabbitmqSource
	Labels               map[string]string
	SinkURI              *apis.URL
	ObservabilityConfig  string
	LoggingConfig        string
	RabbitMQSecretName   string
	RabbitMQCASecretName string
	BrokerUrlSecretKey   string
	ResourceRequirements corev1.ResourceRequirements
}

func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
	replicas := int32(1)
	env := []corev1.EnvVar{
		{
			Name: "RABBIT_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: args.RabbitMQSecretName,
					},
					Key: args.BrokerUrlSecretKey,
				},
			},
		},
		{
			Name:  "RABBITMQ_CHANNEL_PARALLELISM",
			Value: strconv.Itoa(*args.Source.Spec.RabbitmqResourcesConfig.Parallelism),
		},
		{
			Name:  "RABBITMQ_EXCHANGE_NAME",
			Value: args.Source.Spec.RabbitmqResourcesConfig.ExchangeName,
		},
		{
			Name:  "RABBITMQ_QUEUE_NAME",
			Value: args.Source.Spec.RabbitmqResourcesConfig.QueueName,
		},
		{
			Name:  "RABBITMQ_PREDECLARED",
			Value: strconv.FormatBool(args.Source.Spec.RabbitmqResourcesConfig.Predeclared),
		},
		{
			Name:  "K_SINK",
			Value: args.SinkURI.String(),
		},
		{
			Name:  "NAME",
			Value: args.Source.Name,
		},
		{
			Name:  "NAMESPACE",
			Value: args.Source.Namespace,
		},
		{
			Name:  "K_LOGGING_CONFIG",
			Value: args.LoggingConfig,
		},
		{
			Name:  "K_OBSERVABILITY_CONFIG",
			Value: args.ObservabilityConfig,
		},
		{
			Name:  "RABBITMQ_VHOST",
			Value: args.Source.Spec.RabbitmqResourcesConfig.Vhost,
		},
	}

	// Default requirements only if none of the requirements are set through annotations
	if len(args.ResourceRequirements.Limits) == 0 && len(args.ResourceRequirements.Requests) == 0 {
		// This resource requests and limits comes from performance testing 1500msgs/s with a parallelism of 1000
		// more info in this issue: https://github.com/knative-extensions/eventing-rabbitmq/issues/703
		args.ResourceRequirements = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi")},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4000m"),
				corev1.ResourceMemory: resource.MustParse("600Mi")},
		}
	}
	if args.Source.Spec.Delivery != nil {
		if args.Source.Spec.Delivery.Retry != nil {
			env = append(env, corev1.EnvVar{
				Name:  "HTTP_SENDER_RETRY",
				Value: strconv.FormatInt(int64(*args.Source.Spec.Delivery.Retry), 10),
			})
		}

		if args.Source.Spec.Delivery.BackoffPolicy != nil {
			env = append(env, corev1.EnvVar{
				Name:  "HTTP_SENDER_BACKOFF_POLICY",
				Value: string(*args.Source.Spec.Delivery.BackoffPolicy),
			})
		} else {
			env = append(env, corev1.EnvVar{
				Name:  "HTTP_SENDER_BACKOFF_POLICY",
				Value: string(eventingduckv1.BackoffPolicyExponential),
			})
		}

		if args.Source.Spec.Delivery.BackoffDelay != nil {
			env = append(env, corev1.EnvVar{
				Name:  "HTTP_SENDER_BACKOFF_DELAY",
				Value: *args.Source.Spec.Delivery.BackoffDelay,
			})
		}
	}

	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:         kmeta.ChildName(fmt.Sprintf("rabbitmqsource-%s-", args.Source.Name), string(args.Source.UID)),
			Namespace:    args.Source.Namespace,
			GenerateName: fmt.Sprintf("%s-", args.Source.Name),
			Labels:       args.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.Labels,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: args.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.Source.Spec.ServiceAccountName,
					Containers: []corev1.Container{{
						Name:            "receive-adapter",
						Image:           args.Image,
						ImagePullPolicy: "IfNotPresent",
						Env:             env,
						// This resource requests and limits comes from performance testing 1500msgs/s with a parallelism of 1000
						// more info in this issue: https://github.com/knative-extensions/eventing-rabbitmq/issues/703
						Resources: args.ResourceRequirements,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.Bool(false),
							ReadOnlyRootFilesystem:   ptr.Bool(true),
							RunAsNonRoot:             ptr.Bool(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
						},
					}},
				},
			},
		},
	}
	if args.RabbitMQCASecretName != "" {
		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "rabbitmq-ca",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: args.RabbitMQCASecretName,
				},
			},
		}}

		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				MountPath: "/etc/ssl/certs/",
				Name:      "rabbitmq-ca",
			}}
	}

	return deployment
}
