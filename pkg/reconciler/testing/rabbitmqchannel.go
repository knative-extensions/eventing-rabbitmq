/*
Copyright 2019 The Knative Authors

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

package testing

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"

	"knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
)

// RabbitmqChannelOption enables further configuration of a RabbitmqChannel.
type RabbitmqChannelOption func(*v1beta1.RabbitmqChannel)

// NewRabbitmqChannel creates a RabbitmqChannel with RabbitmqChannelOptions.
func NewRabbitmqChannel(name, namespace string, ncopt ...RabbitmqChannelOption) *v1beta1.RabbitmqChannel {
	nc := &v1beta1.RabbitmqChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.RabbitmqChannelSpec{},
	}
	for _, opt := range ncopt {
		opt(nc)
	}
	nc.SetDefaults(context.Background())
	return nc
}

// WithReady marks a NatssChannel as being ready
// The dispatcher reconciler does not set the ready status, instead the controller reconciler does
// For testing, we need to be able to set the status to ready
func WithReady(nc *v1beta1.RabbitmqChannel) {
	cs := apis.NewLivingConditionSet()
	cs.Manage(&nc.Status).MarkTrue(v1beta1.RabbitmqChannelConditionReady)
}

func WithNotReady(reason, messageFormat string) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		cs := apis.NewLivingConditionSet()
		cs.Manage(&nc.Status).MarkFalse(v1beta1.RabbitmqChannelConditionReady, reason, messageFormat)
	}
}

func WithRabbitmqInitChannelConditions(nc *v1beta1.RabbitmqChannel) {
	nc.Status.InitializeConditions()
}

func WithRabbitmqChannelFinalizer(nc *v1beta1.RabbitmqChannel) {
	nc.Finalizers = []string{"rabbitmq-ch-dispatcher"}
}

func WithRabbitmqChannelDeleted(nc *v1beta1.RabbitmqChannel) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	nc.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithRabbitmqChannelDeploymentNotReady(reason, message string) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.MarkDispatcherFailed(reason, message)
	}
}

func WithRabbitmqChannelDeploymentReady() RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.PropagateDispatcherStatus(&appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}})
	}
}

func WithRabbitmqChannelServiceNotReady(reason, message string) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.MarkServiceFailed(reason, message)
	}
}

func WithRabbitmqChannelServiceReady() RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.MarkServiceTrue()
	}
}

func WithRabbitmqChannelChannelServicetNotReady(reason, message string) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.MarkChannelServiceFailed(reason, message)
	}
}

func WithRabbitmqChannelChannelServiceReady() RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.MarkChannelServiceTrue()
	}
}

func WithRabbitmqChannelEndpointsNotReady(reason, message string) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.MarkEndpointsFailed(reason, message)
	}
}

func WithRabbitmqChannelEndpointsReady() RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.MarkEndpointsTrue()
	}
}

func WithRabbitmqChannelSubscriber(t *testing.T, subscriberURI string) RabbitmqChannelOption {
	s, err := apis.ParseURL(subscriberURI)
	if err != nil {
		t.Errorf("cannot parse url: %v", err)
	}
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Spec.Subscribers = []duckv1.SubscriberSpec{{
			UID:           "",
			Generation:    0,
			SubscriberURI: s,
		}}
	}
}

func WithRabbitmqChannelSubscribers(subscribers []duckv1.SubscriberSpec) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Spec.Subscribers = subscribers
	}
}

func WithRabbitmqChannelSubscribableStatus(ready corev1.ConditionStatus, message string) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.Subscribers = []duckv1.SubscriberStatus{{
			Ready:   ready,
			Message: message,
		}}
	}
}
func WithRabbitmqChannelReadySubscriber(uid string) RabbitmqChannelOption {
	return WithRabbitmqChannelReadySubscriberAndGeneration(uid, 0)
}

func WithRabbitmqChannelReadySubscriberAndGeneration(uid string, observedGeneration int64) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.Subscribers = append(nc.Status.Subscribers, duckv1.SubscriberStatus{
			UID:                types.UID(uid),
			ObservedGeneration: observedGeneration,
			Ready:              corev1.ConditionTrue,
		})
	}
}

func WithRabbitmqChannelAddress(a string) RabbitmqChannelOption {
	return func(nc *v1beta1.RabbitmqChannel) {
		nc.Status.SetAddress(&apis.URL{
			Scheme: "http",
			Host:   a,
		})
	}
}

func Addressable() RabbitmqChannelOption {
	return func(channel *v1beta1.RabbitmqChannel) {
		channel.GetConditionSet().Manage(&channel.Status).MarkTrue(v1beta1.RabbitmqChannelConditionAddressable)
	}
}
