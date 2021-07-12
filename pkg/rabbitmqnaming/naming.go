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

	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

// BrokerExchangeName creates a name for Broker Exchange.
// Format is b.Namespace.Name.BrokerUID for normal exchanges and
// b.Namespace.Name.dlx.BrokerUID for DLX exchanges.
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

// ChannelExchangeName creates a name for Channel Exchange.
// Format is c.Namespace.Name.ChannelUID for normal exchanges and
// c.Namespace.Name.dlx.ChannelUID for DLX exchanges.
func ChannelExchangeName(c *messagingv1beta1.RabbitmqChannel, dlx bool) string {
	var exchangeBase string
	if dlx {
		exchangeBase = fmt.Sprintf("c.%s.%s.dlx.", c.Namespace, c.Name)
	} else {
		exchangeBase = fmt.Sprintf("c.%s.%s.", c.Namespace, c.Name)

	}
	return kmeta.ChildName(exchangeBase, string(c.GetUID()))
}

// CreateChannelDeadLetterQueueName constructs a Channel dead letter queue name.
// Format is c.Namespace.Name.dlq.ChannelUID
func CreateChannelDeadLetterQueueName(c *messagingv1beta1.RabbitmqChannel) string {
	dlqBase := fmt.Sprintf("c.%s.%s.dlq.", c.Namespace, c.Name)
	return kmeta.ChildName(dlqBase, string(c.GetUID()))
}

// SubscriberDLXExchangeName creates a DLX name that's used if Subscription has defined
// a DeadLetterSink.
// Format is s.ChannelNamespace.ChannelName.dlx.SubscriptionUID
func SubscriberDLXExchangeName(c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) string {
	exchangeBase := fmt.Sprintf("s.%s.%s.dlx.", c.Namespace, c.Name)
	return kmeta.ChildName(exchangeBase, string(s.UID))
}

// CreateSubscriberQueueName creates a queue name for Subscription events.
// Format is s.ChannelNamespace.ChannelName.SubscriptionUID
func CreateSubscriberQueueName(c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) string {
	subscriptionQueueBase := fmt.Sprintf("s.%s.%s.", c.Namespace, c.Name)
	return kmeta.ChildName(subscriptionQueueBase, string(s.UID))
}

// CreateSubsriberDeadLetterQueueName creates a DLQ name for Subscription events.
// Format is s.ChannelNamespace.ChannelName.dlq.SubscriptionUID
func CreateSubscriberDeadLetterQueueName(c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) string {
	subscriptionDLQBase := fmt.Sprintf("s.%s.%s.dlq.", c.Namespace, c.Name)
	return kmeta.ChildName(subscriptionDLQBase, string(s.UID))
}
