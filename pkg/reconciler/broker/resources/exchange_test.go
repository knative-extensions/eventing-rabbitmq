package resources_test

import (
	"context"
	"testing"

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	brokerName      = "testbroker"
	triggerName     = "testtrigger"
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
	for _, tt := range []struct {
		name    string
		trigger *eventingv1.Trigger
		dlx     bool
		want    *rabbitv1beta1.Exchange
	}{{
		name: "broker exchange",
		want: &rabbitv1beta1.Exchange{
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
		},
	}, {
		name: "broker DLX exchange",
		dlx:  true,
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "broker.foobar.testbroker.dlx",
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
				Name:       "broker.foobar.testbroker.dlx",
				Type:       "headers",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}, {
		name: "trigger DLX exchange",
		dlx:  true,
		trigger: &eventingv1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      triggerName,
			},
		},
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "trigger.foobar.testtrigger.dlx",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Trigger",
						APIVersion: "eventing.knative.dev/v1",
						Name:       triggerName,
					},
				},
				Labels: map[string]string{
					"eventing.knative.dev/broker":  "testbroker",
					"eventing.knative.dev/trigger": "testtrigger",
				},
			},
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       "trigger.foobar.testtrigger.dlx",
				Type:       "headers",
				Durable:    true,
				AutoDelete: false,
				RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
					Name: rabbitmqcluster,
				},
			},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			args := &resources.ExchangeArgs{
				Broker: &eventingv1.Broker{
					ObjectMeta: metav1.ObjectMeta{
						Name:      brokerName,
						Namespace: namespace,
					},
					Spec: eventingv1.BrokerSpec{
						Config: &duckv1.KReference{
							Name: rabbitmqcluster,
						},
					},
				},
				RabbitMQCluster: rabbitmqcluster,
				Trigger:         tt.trigger,
				DLX:             tt.dlx,
			}

			got := resources.NewExchange(context.TODO(), args)
			if !equality.Semantic.DeepDerivative(tt.want, got) {
				t.Errorf("Unexpected Exchange resource: want:\n%+v\ngot:\n%+v", tt.want, got)
			}
		})
	}
}
