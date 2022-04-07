package rabbit

import (
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	TriggerLabelKey = "eventing.knative.dev/trigger"
	SourceLabelKey  = "eventing.knative.dev/SourceName"
)

// Labels generates the labels for a RabbitMQ resource
// Used by exchanges, queues, and bindings created by broker, trigger, and source controllers
func Labels(b *eventingv1.Broker, t *eventingv1.Trigger, s *v1alpha1.RabbitmqSource) map[string]string {
	if t != nil {
		return map[string]string{
			eventing.BrokerLabelKey: b.Name,
			TriggerLabelKey:         t.Name,
		}
	} else if b != nil {
		return map[string]string{
			eventing.BrokerLabelKey: b.Name,
		}
	} else if s != nil {
		return map[string]string{
			SourceLabelKey: s.Name,
		}
	}
	return nil
}
