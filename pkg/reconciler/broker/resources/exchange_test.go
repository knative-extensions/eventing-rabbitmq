package resources_test

import (
	"context"
	"testing"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	brokerName      = "testbroker"
	namespace       = "foobar"
	rabbitmqcluster = "testrabbitmqcluster"
)

func TestExchangeName(t *testing.T) {
	for _, tt := range []struct {
		name      string
		namespace string
		dlx       bool
		want      string
	}{{
		name:      brokerName,
		namespace: namespace,
		want:      "broker.foobar.testbroker",
	}, {
		name:      brokerName,
		namespace: namespace,
		want:      "broker.foobar.testbroker.dlx",
		dlx:       true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resources.ExchangeName(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{Namespace: tt.namespace, Name: tt.name}}, tt.dlx)
			if got != tt.want {
				t.Errorf("Unexpected name for %s/%s DLX: %t: want:\n%+s\ngot:\n%+s", tt.namespace, tt.name, tt.dlx, tt.want, got)
			}
		})
	}
}

func TestTriggerDLXExchangeName(t *testing.T) {
	want := "trigger.foobar.testtrigger.dlx"
	got := resources.TriggerDLXExchangeName(&eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foobar",
			Name:      "testtrigger",
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/testtrigger Trigger DLX: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestNewExchange(t *testing.T) {
	want := &rabbitv1beta1.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "broker.foobar.testbroker",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "Broker",
					APIVersion: "eventing.knative.dev/v1",
					Name:       brokerName,
				},
			},
			Labels: map[string]string{"eventing.knative.dev/broker": "testbroker"},
		},
		Spec: rabbitv1beta1.ExchangeSpec{
			Name:       "broker.foobar.testbroker",
			Type:       "headers",
			Durable:    true,
			AutoDelete: false,
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: rabbitmqcluster,
			},
		},
	}
	args := &resources.ExchangeArgs{
		Broker: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerName,
				Namespace: namespace,
			},
		},
		RabbitMQCluster: rabbitmqcluster,
	}
	got := resources.NewExchange(context.TODO(), args)
	if !equality.Semantic.DeepDerivative(want, got) {
		t.Errorf("Unexpected Exchange resource: want:\n%+v\ngot:\n%+v", want, got)
	}
}
