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

package rabbit_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

func TestLabels(t *testing.T) {
	var (
		brokerName  = "testbroker"
		triggerName = "testtrigger"
		sourceName  = "a-source"
	)

	for _, tt := range []struct {
		name string
		b    *eventingv1.Broker
		t    *eventingv1.Trigger
		s    *v1alpha1.RabbitmqSource
		want map[string]string
	}{{
		name: "broker labels",
		b: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name: brokerName,
			},
		},
		want: map[string]string{
			"eventing.knative.dev/broker": brokerName,
		},
	}, {
		name: "trigger labels",
		b: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name: brokerName,
			},
		},
		t: &eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name: triggerName,
			},
		},
		want: map[string]string{
			"eventing.knative.dev/broker":  brokerName,
			"eventing.knative.dev/trigger": triggerName,
		},
	}, {
		name: "source labels",
		s: &v1alpha1.RabbitmqSource{
			ObjectMeta: metav1.ObjectMeta{
				Name: sourceName,
			},
		},
		want: map[string]string{
			"eventing.knative.dev/SourceName": sourceName,
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got := rabbit.Labels(tt.b, tt.t, tt.s)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected maps of Labels: want:\n%+v\ngot:\n%+v\ndiff:\n%+v", tt.want, got, cmp.Diff(tt.want, got))
			}
		})
	}
}
