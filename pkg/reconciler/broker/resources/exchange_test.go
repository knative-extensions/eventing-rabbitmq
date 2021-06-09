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
	brokerUID       = "broker-test-uid"
	triggerName     = "testtrigger"
	triggerUID      = "trigger-test-uid"
	namespace       = "foobar"
	rabbitmqcluster = "testrabbitmqcluster"
)

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
				Name:      "b.foobar.testbroker.broker-test-uid",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Broker",
						APIVersion: "eventing.knative.dev/v1",
						Name:       brokerName,
						UID:        brokerUID,
					},
				},
				Labels: map[string]string{"eventing.knative.dev/broker": "testbroker"},
			},
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       "b.foobar.testbroker.broker-test-uid",
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
				Name:      "b.foobar.testbroker.dlx.broker-test-uid",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Broker",
						APIVersion: "eventing.knative.dev/v1",
						Name:       brokerName,
						UID:        brokerUID,
					},
				},
				Labels: map[string]string{"eventing.knative.dev/broker": "testbroker"},
			},
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       "b.foobar.testbroker.dlx.broker-test-uid",
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
				UID:       triggerUID,
			},
		},
		want: &rabbitv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "t.foobar.testtrigger.dlx.trigger-test-uid",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Trigger",
						APIVersion: "eventing.knative.dev/v1",
						Name:       triggerName,
						UID:        triggerUID,
					},
				},
				Labels: map[string]string{
					"eventing.knative.dev/broker":  "testbroker",
					"eventing.knative.dev/trigger": "testtrigger",
				},
			},
			Spec: rabbitv1beta1.ExchangeSpec{
				Name:       "t.foobar.testtrigger.dlx.trigger-test-uid",
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
						UID:       brokerUID,
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
