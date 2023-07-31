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
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

const (
	brokerName       = "testbroker"
	triggerName      = "testtrigger"
	ns               = "testnamespace"
	image            = "dispatcherimage"
	secretName       = "testbroker-broker-rabbit"
	brokerURLKey     = "testbrokerurl"
	queueName        = "testnamespace-testtrigger"
	brokerIngressURL = "http://broker.example.com"
	subscriberURL    = "http://function.example.com"
)

var (
	exponentialBackoff = eventingduckv1.BackoffPolicyExponential
	testCACert         = "test.cacert"
)

func TestMakeDispatcherDeployment(t *testing.T) {
	tests := []struct {
		name string
		args *DispatcherArgs
		want *appsv1.Deployment
	}{
		{
			name: "base",
			args: dispatcherArgs(),
			want: deployment(withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}), withDefaultResourceRequirements()),
		},
		{
			name: "with delivery spec",
			args: dispatcherArgs(withDelivery(&eventingduckv1.DeliverySpec{
				Retry:         Int32Ptr(10),
				BackoffPolicy: &exponentialBackoff,
				BackoffDelay:  ptr.String("PT20S"),
				Timeout:       ptr.String("PT10S"),
			})),
			want: deployment(
				withEnv(corev1.EnvVar{Name: "RETRY", Value: "10"}),
				withEnv(corev1.EnvVar{Name: "BACKOFF_POLICY", Value: "exponential"}),
				withEnv(corev1.EnvVar{Name: "BACKOFF_DELAY", Value: "20s"}),
				withEnv(corev1.EnvVar{Name: "TIMEOUT", Value: "10s"}),
				withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}),
				withDefaultResourceRequirements(),
			),
		},
		{
			name: "with dlx",
			args: dispatcherArgs(withDLX),
			want: deployment(
				deploymentNamed("testtrigger-dlx-dispatcher"),
				withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}),
				withEnv(corev1.EnvVar{Name: "DLX", Value: "true"}),
				withEnv(corev1.EnvVar{Name: "POD_NAME", Value: "testtrigger-dlx-dispatcher"}),
				withDefaultResourceRequirements(),
			),
		}, {
			name: "with dlx name",
			args: dispatcherArgs(withDLXName("dlx-name")),
			want: deployment(
				deploymentNamed("testtrigger-dispatcher"),
				withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}),
				withEnv(corev1.EnvVar{Name: "DLX_NAME", Value: "dlx-name"}),
				withEnv(corev1.EnvVar{Name: "POD_NAME", Value: "testtrigger-dispatcher"}),
				withDefaultResourceRequirements(),
			),
		},
		{
			name: "with parallelism",
			args: dispatcherArgs(withParallelism("10")),
			want: deployment(withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "10"}), withDefaultResourceRequirements()),
		},
		{
			name: "with vhost",
			args: dispatcherArgs(withVhost()),
			want: deployment(
				withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}),
				withEnv(corev1.EnvVar{Name: "RABBITMQ_VHOST", Value: "test-vhost"}),
				withDefaultResourceRequirements()),
		},
		{
			name: "with resource requirements",
			args: dispatcherArgs(
				withResourceArgs(corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"key1": resource.MustParse("5m"),
						"key2": resource.MustParse("10m"),
					},
					Limits: corev1.ResourceList{
						"key3": resource.MustParse("15m"),
					},
				}),
			),
			want: deployment(
				withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}),
				withResourceRequirements(corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"key1": resource.MustParse("5m"),
						"key2": resource.MustParse("10m"),
					},
					Limits: corev1.ResourceList{
						"key3": resource.MustParse("15m"),
					},
				}),
			),
		}, {
			name: "with subscriber CACerts",
			args: dispatcherArgs(withSubscriberCACerts()),
			want: deployment(
				withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}),
				withEnv(corev1.EnvVar{Name: "SUBSCRIBER_CACERTS", Value: testCACert}),
				withDefaultResourceRequirements()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeDispatcherDeployment(tt.args)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Error("unexpected diff (-want, +got) = ", diff)
			}
		})
	}
}

func deployment(opts ...func(*appsv1.Deployment)) *appsv1.Deployment {
	trueValue := true
	one := int32(1)
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "testtrigger-dispatcher",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Trigger",
				Name:               triggerName,
				Controller:         &trueValue,
				BlockOwnerDeletion: &trueValue,
			}},
			Labels: map[string]string{
				"eventing.knative.dev/broker":     brokerName,
				"eventing.knative.dev/brokerRole": "dispatcher",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/broker":     brokerName,
					"eventing.knative.dev/brokerRole": "dispatcher",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"eventing.knative.dev/broker":     brokerName,
						"eventing.knative.dev/brokerRole": "dispatcher",
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
							Name:  "BROKER_INGRESS_URL",
							Value: brokerIngressURL,
						}, {
							Name:  "NAMESPACE",
							Value: ns,
						}, {
							Name:  "CONTAINER_NAME",
							Value: dispatcherContainerName,
						}, {
							Name:  "POD_NAME",
							Value: "testtrigger-dispatcher",
						}},
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
					}},
				},
			},
		},
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

func withEnv(env corev1.EnvVar) func(*appsv1.Deployment) {
	return func(d *appsv1.Deployment) {
		found := false
		for i, e := range d.Spec.Template.Spec.Containers[0].Env {
			if e.Name == env.Name {
				d.Spec.Template.Spec.Containers[0].Env[i].Value = env.Value
				found = true
			}
		}

		if !found {
			d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, env)
		}
	}
}

func deploymentNamed(name string) func(*appsv1.Deployment) {
	return func(d *appsv1.Deployment) {
		d.ObjectMeta.Name = name
	}
}

func dispatcherArgs(opts ...func(*DispatcherArgs)) *DispatcherArgs {
	ingressURL := apis.HTTP("broker.example.com")
	sURL := apis.HTTP("function.example.com")
	sAddressable := &duckv1.Addressable{
		URL: sURL,
		// CACerts: , still to be implemented
	}
	trigger := &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{Name: triggerName, Namespace: ns},
		Spec:       eventingv1.TriggerSpec{Broker: brokerName},
	}
	args := &DispatcherArgs{
		Trigger:              trigger,
		Image:                image,
		RabbitMQSecretName:   secretName,
		RabbitMQCASecretName: "rabbitmq-ca-secret",
		QueueName:            queueName,
		BrokerUrlSecretKey:   brokerURLKey,
		BrokerIngressURL:     ingressURL,
		Subscriber:           sAddressable,
	}
	for _, o := range opts {
		o(args)
	}
	return args
}

func withDelivery(delivery *eventingduckv1.DeliverySpec) func(*DispatcherArgs) {
	return func(args *DispatcherArgs) {
		args.Delivery = delivery
	}
}

func withDLX(args *DispatcherArgs) {
	args.DLX = true
}

func withDLXName(name string) func(*DispatcherArgs) {
	return func(args *DispatcherArgs) {
		args.DLXName = name
	}
}

func withParallelism(c string) func(*DispatcherArgs) {
	return func(args *DispatcherArgs) {
		if args.Trigger.ObjectMeta.Annotations == nil {
			args.Trigger.ObjectMeta.Annotations = map[string]string{ParallelismAnnotation: c}
		} else {
			args.Trigger.ObjectMeta.Annotations[ParallelismAnnotation] = c
		}
	}
}

func withResourceArgs(req corev1.ResourceRequirements) func(*DispatcherArgs) {
	return func(args *DispatcherArgs) {
		args.ResourceRequirements = req
	}
}

func withResourceRequirements(req corev1.ResourceRequirements) func(*appsv1.Deployment) {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Resources = req
	}
}

func withDefaultResourceRequirements() func(*appsv1.Deployment) {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi")},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4000m"),
				corev1.ResourceMemory: resource.MustParse("600Mi")},
		}
	}
}

func withVhost() func(*DispatcherArgs) {
	return func(args *DispatcherArgs) {
		args.RabbitMQVHost = "test-vhost"
	}
}

func withSubscriberCACerts() func(*DispatcherArgs) {
	return func(args *DispatcherArgs) {
		args.Subscriber.CACerts = &testCACert
	}
}

func Int32Ptr(i int32) *int32 {
	return &i
}
