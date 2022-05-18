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

	"github.com/rickb777/date/period"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

type ReceiveAdapterArgs struct {
	Image         string
	Source        *v1alpha1.RabbitmqSource
	Labels        map[string]string
	SinkURI       string
	MetricsConfig string
	LoggingConfig string
}

func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
	replicas := int32(1)

	env := []corev1.EnvVar{
		{
			Name:  "RABBITMQ_BROKERS",
			Value: args.Source.Spec.Broker,
		},
		{
			Name: "RABBITMQ_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: args.Source.Spec.User.SecretKeyRef,
			},
		},
		{
			Name: "RABBITMQ_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: args.Source.Spec.Password.SecretKeyRef,
			},
		},
		{
			Name:  "RABBITMQ_ROUTING_KEY",
			Value: args.Source.Spec.QueueConfig.RoutingKey,
		},
		{
			Name:  "RABBITMQ_CHANNEL_CONFIG_QOS_GLOBAL",
			Value: strconv.FormatBool(args.Source.Spec.ChannelConfig.GlobalQos),
		},
		{
			Name:  "RABBITMQ_CHANNEL_CONFIG_PARALLELISM",
			Value: strconv.Itoa(*args.Source.Spec.ChannelConfig.Parallelism),
		},
		{
			Name:  "RABBITMQ_EXCHANGE_CONFIG_NAME",
			Value: args.Source.Spec.ExchangeConfig.Name,
		},
		{
			Name:  "RABBITMQ_EXCHANGE_CONFIG_TYPE",
			Value: args.Source.Spec.ExchangeConfig.Type,
		},
		{
			Name:  "RABBITMQ_EXCHANGE_CONFIG_DURABLE",
			Value: strconv.FormatBool(args.Source.Spec.ExchangeConfig.Durable),
		},
		{
			Name:  "RABBITMQ_EXCHANGE_CONFIG_AUTO_DELETE",
			Value: strconv.FormatBool(args.Source.Spec.ExchangeConfig.AutoDelete),
		},
		{
			Name:  "RABBITMQ_QUEUE_CONFIG_NAME",
			Value: args.Source.Spec.QueueConfig.Name,
		},
		{
			Name:  "RABBITMQ_QUEUE_CONFIG_DURABLE",
			Value: strconv.FormatBool(args.Source.Spec.QueueConfig.Durable),
		},
		{
			Name:  "RABBITMQ_QUEUE_CONFIG_AUTO_DELETE",
			Value: strconv.FormatBool(args.Source.Spec.QueueConfig.AutoDelete),
		},
		{
			Name:  "RABBITMQ_PREDECLARED",
			Value: strconv.FormatBool(args.Source.Spec.Predeclared),
		},
		{
			Name:  "SINK_URI",
			Value: args.SinkURI,
		},
		{
			Name:  "K_SINK",
			Value: args.SinkURI,
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
			Name:  "K_METRICS_CONFIG",
			Value: args.MetricsConfig,
		},
		{
			Name:  "RABBITMQ_VHOST",
			Value: args.Source.Spec.Vhost,
		},
	}

	if args.Source.Spec.Retry != nil {
		env = append(env, corev1.EnvVar{
			Name:  "HTTP_SENDER_RETRY",
			Value: strconv.FormatInt(int64(*args.Source.Spec.Retry), 10),
		})
	}

	if args.Source.Spec.BackoffPolicy != nil {
		env = append(env, corev1.EnvVar{
			Name:  "HTTP_SENDER_BACKOFF_POLICY",
			Value: string(*args.Source.Spec.BackoffPolicy),
		})
	} else {
		env = append(env, corev1.EnvVar{
			Name:  "HTTP_SENDER_BACKOFF_POLICY",
			Value: string(eventingduckv1.BackoffPolicyExponential),
		})
	}

	if args.Source.Spec.BackoffDelay != nil {
		p, _ := period.Parse(*args.Source.Spec.BackoffDelay)
		env = append(env, corev1.EnvVar{
			Name:  "HTTP_SENDER_BACKOFF_DELAY",
			Value: p.DurationApprox().String(),
		})
	}

	return &v1.Deployment{
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
					Containers: []corev1.Container{
						{
							Name:            "receive-adapter",
							Image:           args.Image,
							ImagePullPolicy: "IfNotPresent",
							Env:             env,
							// This resource requests and limits comes from performance testing 1500msgs/s with a parallelism of 1000
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("300m"),
									corev1.ResourceMemory: resource.MustParse("15Mi")},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("4000m"),
									corev1.ResourceMemory: resource.MustParse("500Mi")},
							},
						},
					},
				},
			},
		},
	}
}
