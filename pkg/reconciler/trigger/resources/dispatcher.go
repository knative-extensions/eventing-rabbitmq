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

	"github.com/rickb777/date/period"
	"k8s.io/apimachinery/pkg/api/resource"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"
)

const (
	dispatcherContainerName = "dispatcher"
	ParallelismAnnotation   = "rabbitmq.eventing.knative.dev/parallelism"
)

// DispatcherArgs are the arguments to create a dispatcher deployment. There's
// one of these created for each trigger.
type DispatcherArgs struct {
	Delivery *eventingduckv1.DeliverySpec
	Trigger  *eventingv1.Trigger
	Image    string
	//ServiceAccountName string
	RabbitMQVHost        string
	RabbitMQSecretName   string
	RabbitMQCASecretName string
	QueueName            string
	BrokerUrlSecretKey   string
	BrokerIngressURL     *apis.URL
	Subscriber           *duckv1.Addressable
	DLX                  bool
	DLXName              string
	Configs              reconcilersource.ConfigAccessor
	ResourceRequirements corev1.ResourceRequirements
}

// MakeDispatcherDeployment creates the in-memory representation of the Broker's Dispatcher Deployment.
func MakeDispatcherDeployment(args *DispatcherArgs) *appsv1.Deployment {
	one := int32(1)
	name := DispatcherName(args.Trigger.Name, args.DLX)

	// Default requirements only if none of the requirements are set through annotations
	if len(args.ResourceRequirements.Limits) == 0 && len(args.ResourceRequirements.Requests) == 0 {
		// This resource requests and limits comes from performance testing 1500msgs/s with a parallelism of 1000
		// more info in this issue: https://github.com/knative-sandbox/eventing-rabbitmq/issues/703
		args.ResourceRequirements = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi")},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4000m"),
				corev1.ResourceMemory: resource.MustParse("600Mi")},
		}
	}

	dispatcher := corev1.Container{
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
			Value: args.Subscriber.URL.String(),
		}, {
			Name:  "BROKER_INGRESS_URL",
			Value: args.BrokerIngressURL.String(),
		}, {
			Name:  "NAMESPACE",
			Value: args.Trigger.Namespace,
		}, {
			Name:  "CONTAINER_NAME",
			Value: dispatcherContainerName,
		}, {
			Name:  "POD_NAME",
			Value: name,
		}},
		Resources: args.ResourceRequirements,
		Ports: []corev1.ContainerPort{{
			Name:          "http-metrics",
			ContainerPort: 9090,
		}},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.Bool(false),
			ReadOnlyRootFilesystem:   ptr.Bool(true),
			RunAsNonRoot:             ptr.Bool(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}
	if args.Configs != nil {
		dispatcher.Env = append(dispatcher.Env, args.Configs.ToEnvVars()...)
	}
	if args.Delivery != nil {
		if args.Delivery.Retry != nil {
			dispatcher.Env = append(dispatcher.Env,
				corev1.EnvVar{
					Name:  "RETRY",
					Value: fmt.Sprint(*args.Delivery.Retry),
				})

		} else {
			dispatcher.Env = append(dispatcher.Env,
				corev1.EnvVar{
					Name:  "RETRY",
					Value: "5",
				})
		}
		if args.Delivery.BackoffPolicy != nil {
			dispatcher.Env = append(dispatcher.Env,
				corev1.EnvVar{
					Name:  "BACKOFF_POLICY",
					Value: string(*args.Delivery.BackoffPolicy),
				})
		}
		if args.Delivery.BackoffDelay != nil {
			p, _ := period.Parse(*args.Delivery.BackoffDelay)
			dispatcher.Env = append(dispatcher.Env,
				corev1.EnvVar{
					Name:  "BACKOFF_DELAY",
					Value: p.DurationApprox().String(),
				})
		}
		if args.Delivery.Timeout != nil {
			timeout, _ := period.Parse(*args.Delivery.Timeout)
			dispatcher.Env = append(dispatcher.Env,
				corev1.EnvVar{
					Name:  "TIMEOUT",
					Value: timeout.DurationApprox().String(),
				})
		}
	}
	if parallelism, ok := args.Trigger.ObjectMeta.Annotations[ParallelismAnnotation]; ok {
		dispatcher.Env = append(dispatcher.Env,
			corev1.EnvVar{
				Name:  "PARALLELISM",
				Value: parallelism,
			})
	} else {
		dispatcher.Env = append(dispatcher.Env,
			corev1.EnvVar{
				Name:  "PARALLELISM",
				Value: "1",
			})
	}
	if args.RabbitMQVHost != "" {
		dispatcher.Env = append(dispatcher.Env,
			corev1.EnvVar{
				Name:  "RABBITMQ_VHOST",
				Value: args.RabbitMQVHost,
			})
	}
	if args.Subscriber.CACerts != nil {
		dispatcher.Env = append(dispatcher.Env,
			corev1.EnvVar{
				Name:  "SUBSCRIBER_CACERTS",
				Value: *args.Subscriber.CACerts,
			})
	}
	if args.DLX {
		dispatcher.Env = append(dispatcher.Env,
			corev1.EnvVar{
				Name:  "DLX",
				Value: "true",
			})
	}
	if args.DLXName != "" {
		dispatcher.Env = append(dispatcher.Env,
			corev1.EnvVar{
				Name:  "DLX_NAME",
				Value: args.DLXName,
			})
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Trigger.Namespace,
			Name:      name,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Trigger),
			},
			Labels: DispatcherLabels(args.Trigger.Spec.Broker),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: DispatcherLabels(args.Trigger.Spec.Broker),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: DispatcherLabels(args.Trigger.Spec.Broker),
				},
				Spec: corev1.PodSpec{
					//ServiceAccountName: args.ServiceAccountName,
					Containers: []corev1.Container{dispatcher},
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

// DispatcherLabels generates the labels present on all resources representing the dispatcher of the given
// Broker.
func DispatcherLabels(brokerName string) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey:           brokerName,
		"eventing.knative.dev/brokerRole": "dispatcher",
	}
}

func DispatcherName(triggerName string, dlq bool) string {
	if dlq {
		return fmt.Sprintf("%s-dlx-dispatcher", triggerName)
	} else {
		return fmt.Sprintf("%s-dispatcher", triggerName)
	}
}
