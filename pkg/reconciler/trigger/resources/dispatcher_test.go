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
	"k8s.io/utils/pointer"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
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
	rabbitHost       = "amqp://localhost.example.com"
	queueName        = "testnamespace-testtrigger"
	brokerIngressURL = "http://broker.example.com"
	subscriberURL    = "http://function.example.com"
)

var exponentialBackoff = eventingduckv1.BackoffPolicyExponential

func TestMakeDispatcherDeployment(t *testing.T) {
	tests := []struct {
		name string
		args *DispatcherArgs
		want *appsv1.Deployment
	}{
		{
			name: "base",
			args: dispatcherArgs(),
			want: deployment(withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"})),
		},
		{
			name: "with delivery spec",
			args: dispatcherArgs(withDelivery(&eventingduckv1.DeliverySpec{
				Retry:         Int32Ptr(10),
				BackoffPolicy: &exponentialBackoff,
				BackoffDelay:  ptr.String("PT20S"),
				Timeout:       pointer.StringPtr("PT10S"),
			})),
			want: deployment(
				withEnv(corev1.EnvVar{Name: "RETRY", Value: "10"}),
				withEnv(corev1.EnvVar{Name: "BACKOFF_POLICY", Value: "exponential"}),
				withEnv(corev1.EnvVar{Name: "BACKOFF_DELAY", Value: "20s"}),
				withEnv(corev1.EnvVar{Name: "TIMEOUT", Value: "10s"}),
				withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}),
			),
		},
		{
			name: "with dlx",
			args: dispatcherArgs(withDLX),
			want: deployment(
				deploymentNamed("testtrigger-dlx-dispatcher"),
				withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "1"}),
			),
		},
		{
			name: "with parallelism",
			args: dispatcherArgs(withParallelism("10")),
			want: deployment(withEnv(corev1.EnvVar{Name: "PARALLELISM", Value: "10"})),
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
					Containers: []corev1.Container{{
						Name:  "dispatcher",
						Image: image,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("300m"),
								corev1.ResourceMemory: resource.MustParse("10Mi")},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4000m"),
								corev1.ResourceMemory: resource.MustParse("400Mi")},
						},
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
						}},
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
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, env)
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
	trigger := &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{Name: triggerName, Namespace: ns},
		Spec:       eventingv1.TriggerSpec{Broker: brokerName},
	}
	args := &DispatcherArgs{
		Trigger:            trigger,
		Image:              image,
		RabbitMQHost:       rabbitHost,
		RabbitMQSecretName: secretName,
		QueueName:          queueName,
		BrokerUrlSecretKey: brokerURLKey,
		BrokerIngressURL:   ingressURL,
		Subscriber:         sURL,
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

func withParallelism(c string) func(*DispatcherArgs) {
	return func(args *DispatcherArgs) {
		if args.Trigger.ObjectMeta.Annotations == nil {
			args.Trigger.ObjectMeta.Annotations = map[string]string{ParallelismAnnotation: c}
		} else {
			args.Trigger.ObjectMeta.Annotations[ParallelismAnnotation] = c
		}
	}
}

func Int32Ptr(i int32) *int32 {
	return &i
}
