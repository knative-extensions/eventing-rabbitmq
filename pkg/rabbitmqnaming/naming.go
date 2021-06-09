/*
Copyright 2021 The Knative Authors

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

package naming

import (
	"fmt"

	"knative.dev/pkg/kmeta"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

// BrokerExchangeName creates a name for Broker Exchange.
// Format is broker.Namespace.Name.BrokerUID for normal exchanges and
// broker.Namespace.Name.dlx.BrokerUID for DLX exchanges.
func BrokerExchangeName(b *eventingv1.Broker, dlx bool) string {
	var exchangeBase string
	if dlx {
		exchangeBase = fmt.Sprintf("b.%s.%s.dlx.", b.Namespace, b.Name)
	} else {
		exchangeBase = fmt.Sprintf("b.%s.%s.", b.Namespace, b.Name)

	}
	return kmeta.ChildName(exchangeBase, string(b.GetUID()))
}

// TriggerDLXExchangeName creates a DLX name that's used if Trigger has defined
// DeadLetterSink.
// Format is t.Namespace.Name.dlx.TriggerUID
func TriggerDLXExchangeName(t *eventingv1.Trigger) string {
	exchangeBase := fmt.Sprintf("t.%s.%s.dlx.", t.Namespace, t.Name)
	return kmeta.ChildName(exchangeBase, string(t.GetUID()))
}

// CreateBrokerDeadLetterQueueName constructs a Broker dead letter queue name.
// Format is b.Namespace.Name.dlq.BrokerUID
func CreateBrokerDeadLetterQueueName(b *eventingv1.Broker) string {
	dlqBase := fmt.Sprintf("b.%s.%s.dlq.", b.Namespace, b.Name)
	return kmeta.ChildName(dlqBase, string(b.GetUID()))
}

// CreateTriggerQueueName creates a queue name for Trigger events.
// Format is t.Namespace.Name.TriggerUID
func CreateTriggerQueueName(t *eventingv1.Trigger) string {
	triggerQueueBase := fmt.Sprintf("t.%s.%s.", t.Namespace, t.Name)
	return kmeta.ChildName(triggerQueueBase, string(t.GetUID()))
}

// CreateTriggerDeadLetterQueueName creates a dead letter queue name for Trigger
// if Trigger has defined a DeadLetterSink.
// Format is t.Namespace.Name.dlq.TriggerUID
func CreateTriggerDeadLetterQueueName(t *eventingv1.Trigger) string {
	triggerDLQBase := fmt.Sprintf("t.%s.%s.dlq.", t.Namespace, t.Name)
	return kmeta.ChildName(triggerDLQBase, string(t.GetUID()))
}
