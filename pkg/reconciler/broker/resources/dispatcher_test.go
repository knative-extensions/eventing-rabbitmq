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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"

	_ "knative.dev/pkg/system/testing"
)

const (
	brokerName       = "testbroker"
	ns               = "testnamespace"
	image            = "dispatcherimage"
	secretName       = "testbroker-broker-rabbit"
	brokerURLKey     = "testbrokerurl"
	queueName        = "testnamespace-testtrigger"
	brokerIngressURL = "http://broker.example.com"
	subscriberURL    = "http://function.example.com"
)

func TestMakeDispatcherDeployment(t *testing.T) {
	TrueValue := true
	cacert := "test.cacert"
	sURL := apis.HTTP("function.example.com")
	sAddressable := &duckv1.Addressable{
		URL:     sURL,
		CACerts: &cacert,
	}
	bURL := apis.HTTP("broker.example.com")
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: brokerName, Namespace: ns},
	}
	linear := v1.BackoffPolicyLinear
	args := &DispatcherArgs{
		Broker:               broker,
		Image:                image,
		RabbitMQVhost:        "test-vhost",
		RabbitMQSecretName:   secretName,
		RabbitMQCASecretName: "rabbitmq-ca-secret",
		QueueName:            queueName,
		BrokerUrlSecretKey:   brokerURLKey,
		Subscriber:           sAddressable,
		BrokerIngressURL:     bURL,
		Delivery: &v1.DeliverySpec{
			Retry:         ptr.Int32(10),
			BackoffDelay:  ptr.String("PT20S"),
			BackoffPolicy: &linear,
		},
		DLX: true,
	}

	got := MakeDispatcherDeployment(args)
	one := int32(1)
	want := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      "testbroker-dlq-dispatcher",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Broker",
				Name:               brokerName,
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
			Labels: map[string]string{
				"eventing.knative.dev/broker":     brokerName,
				"eventing.knative.dev/brokerRole": "dispatcher-dlq",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/broker":     brokerName,
					"eventing.knative.dev/brokerRole": "dispatcher-dlq",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"eventing.knative.dev/broker":     brokerName,
						"eventing.knative.dev/brokerRole": "dispatcher-dlq",
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "rabbitmq-ca",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "rabbitmq-ca-secret",
							},
						},
					}},
					Containers: []corev1.Container{{
						Name:  "dispatcher",
						Image: image,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("64Mi")},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4000m"),
								corev1.ResourceMemory: resource.MustParse("600Mi")},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.Bool(false),
							ReadOnlyRootFilesystem:   ptr.Bool(true),
							RunAsNonRoot:             ptr.Bool(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								MountPath: "/etc/ssl/certs/",
								Name:      "rabbitmq-ca",
							}},
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
							Name:  "BROKER_INGRESS_URL",
							Value: brokerIngressURL,
						}, {
							Name:  "NAMESPACE",
							Value: args.Broker.Namespace,
						}, {
							Name:  "CONTAINER_NAME",
							Value: dispatcherContainerName,
						}, {
							Name:  "POD_NAME",
							Value: DispatcherName(args.Broker.Name),
						}, {
							Name:  "RETRY",
							Value: "10",
						}, {
							Name:  "BACKOFF_POLICY",
							Value: "linear",
						}, {
							Name:  "BACKOFF_DELAY",
							Value: "20s",
						}, {
							Name:  "RABBITMQ_VHOST",
							Value: "test-vhost",
						}, {
							Name:  "SUBSCRIBER_CACERTS",
							Value: "test.cacert",
						}, {
							Name:  "DLX",
							Value: "true",
						}},
						Ports: []corev1.ContainerPort{{
							Name:          "http-metrics",
							ContainerPort: 9090,
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
