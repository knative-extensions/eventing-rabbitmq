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

package broker

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// BrokerOption enables further configuration of a Broker.
type BrokerOption func(*v1.Broker)

// NewBroker creates a Broker with BrokerOptions.
func NewBroker(name, namespace string, o ...BrokerOption) *v1.Broker {
	v1.RegisterAlternateBrokerConditionSet(rabbitBrokerCondSet)
	b := &v1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(b)
	}
	b.SetDefaults(context.Background())
	return b
}

// WithInitBrokerConditions initializes the Broker's conditions.
func WithInitBrokerConditions(b *v1.Broker) {
	b.Status.InitializeConditions()
}

func WithBrokerFinalizers(finalizers ...string) BrokerOption {
	return func(b *v1.Broker) {
		b.Finalizers = finalizers
	}
}

func WithBrokerResourceVersion(rv string) BrokerOption {
	return func(b *v1.Broker) {
		b.ResourceVersion = rv
	}
}

func WithBrokerGeneration(gen int64) BrokerOption {
	return func(s *v1.Broker) {
		s.Generation = gen
	}
}

// WithBrokerUID sets the UID for a Broker. Handy for testing ownerrefs.
func WithBrokerUID(uid string) BrokerOption {
	return func(b *v1.Broker) {
		b.UID = types.UID(uid)
	}
}

func WithBrokerStatusObservedGeneration(gen int64) BrokerOption {
	return func(s *v1.Broker) {
		s.Status.ObservedGeneration = gen
	}
}

func WithBrokerDelivery(d *eventingduckv1.DeliverySpec) BrokerOption {
	return func(b *v1.Broker) {
		b.Spec.Delivery = d
	}
}

func WithBrokerDeletionTimestamp(b *v1.Broker) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	b.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithBrokerChannel sets the Broker's ChannelTemplateSpec to the specified CRD.
func WithBrokerConfig(config *duckv1.KReference) BrokerOption {
	return func(b *v1.Broker) {
		b.Spec.Config = config
	}
}

// WithBrokerAddress sets the Broker's address.
func WithBrokerAddress(address string) BrokerOption {
	return func(b *v1.Broker) {
		b.Status.SetAddress(&apis.URL{
			Scheme: "http",
			Host:   address,
		})
	}
}

// WithBrokerAddressURI sets the Broker's address as URI.
func WithBrokerAddressURI(uri *apis.URL) BrokerOption {
	return func(b *v1.Broker) {
		b.Status.SetAddress(uri)
	}
}

// WithBrokerReady sets .Status to ready.
func WithBrokerReady(b *v1.Broker) {
	b.Status = *v1.TestHelper.ReadyBrokerStatus()
}

// WithExchangeFailed sets exchange condition to failed.
func WithExchangeFailed(reason, msg string) BrokerOption {
	return func(b *v1.Broker) {
		MarkExchangeFailed(&b.Status, reason, msg)
	}
}

// WithExchangeReady sets exchange condition to ready.
func WithExchangeReady() BrokerOption {
	return func(b *v1.Broker) {
		MarkExchangeReady(&b.Status)
	}
}

// WithDLXFailed() sets DLX condition to failed.
func WithDLXFailed(reason, msg string) BrokerOption {
	return func(b *v1.Broker) {
		MarkDLXFailed(&b.Status, reason, msg)
	}
}

// WithDLXReady() sets DLX condition to ready.
func WithDLXReady() BrokerOption {
	return func(b *v1.Broker) {
		MarkDLXReady(&b.Status)
	}
}

// WithDeadLetterSinkReady() sets DeadLetterSink condition to ready.
func WithDeadLetterSinkReady() BrokerOption {
	return func(b *v1.Broker) {
		MarkDeadLetterSinkReady(&b.Status)
	}
}

// WithDeadLetterSinkFailed sets secret condition to ready.
func WithDeadLetterSinkFailed(reason, msg string) BrokerOption {
	return func(b *v1.Broker) {
		MarkDeadLetterSinkFailed(&b.Status, reason, msg)
	}
}

// WithSecretReady sets secret condition to ready.
func WithSecretReady() BrokerOption {
	return func(b *v1.Broker) {
		MarkSecretReady(&b.Status)
	}
}

// WithSecretFailed sets secret condition to ready.
func WithSecretFailed(reason, msg string) BrokerOption {
	return func(b *v1.Broker) {
		MarkSecretFailed(&b.Status, reason, msg)
	}
}

// WithIngressFailed calls .Status.MarkIngressFailed on the Broker.
func WithIngressFailed(reason, msg string) BrokerOption {
	return func(b *v1.Broker) {
		MarkIngressFailed(&b.Status, reason, msg)
	}
}

func WithIngressAvailable() BrokerOption {
	return func(b *v1.Broker) {
		b.Status.PropagateIngressAvailability(v1.TestHelper.AvailableEndpoints())
	}
}

func WithBrokerClass(bc string) BrokerOption {
	return func(b *v1.Broker) {
		annotations := b.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations[broker.ClassAnnotationKey] = bc
		b.SetAnnotations(annotations)
	}
}
