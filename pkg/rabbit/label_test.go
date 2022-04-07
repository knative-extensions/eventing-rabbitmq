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
