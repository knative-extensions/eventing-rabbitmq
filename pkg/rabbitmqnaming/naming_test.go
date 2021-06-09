package naming

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	brokerName      = "testbroker"
	brokerUID       = "brokeruid"
	triggerName     = "testtrigger"
	triggerUID      = "triggeruid"
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
		want:      "b.foobar.testbroker.brokeruid",
	}, {
		name:      brokerName,
		namespace: namespace,
		want:      "b.foobar.testbroker.dlx.brokeruid",
		dlx:       true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got := BrokerExchangeName(&eventingv1.Broker{ObjectMeta: metav1.ObjectMeta{UID: brokerUID, Namespace: tt.namespace, Name: tt.name}}, tt.dlx)
			if got != tt.want {
				t.Errorf("Unexpected name for Broker Exchange %s/%s DLX: %t: want:\n%+s\ngot:\n%+s", tt.namespace, tt.name, tt.dlx, tt.want, got)
			}
		})
	}
}

func TestTriggerDLXExchangeName(t *testing.T) {
	want := "t.foobar.testtrigger.dlx.triggeruid"
	got := TriggerDLXExchangeName(&eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
			UID:       triggerUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/testtrigger Trigger DLX: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateBrokerDeadLetterQueueName(t *testing.T) {
	want := "b.foobar.testbroker.dlq.brokeruid"
	got := CreateBrokerDeadLetterQueueName(&eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      brokerName,
			UID:       brokerUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/testbroker Broker DLQ: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateTriggerQueueName(t *testing.T) {
	want := "t.foobar.testtrigger.triggeruid"
	got := CreateTriggerQueueName(&eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
			UID:       triggerUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/testtrigger Queue: want:\n%q\ngot:\n%q", want, got)
	}
}

func TestCreateTriggerDeadLetterQueueName(t *testing.T) {
	want := "t.foobar.testtrigger.dlq.triggeruid"
	got := CreateTriggerDeadLetterQueueName(&eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      triggerName,
			UID:       triggerUID,
		},
	})
	if want != got {
		t.Errorf("Unexpected name for foobar/trigger Trigger DLQ: want:\n%q\ngot:\n%q", want, got)
	}
}
