/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package broker

import (
	corev1 "k8s.io/api/core/v1"

	"knative.dev/eventing/pkg/apis/duck"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
)

func MarkIngressFailed(bs *eventingv1.BrokerStatus, reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionIngress, reason, format, args...)
}

func PropagateIngressAvailability(bs *eventingv1.BrokerStatus, ep *corev1.Endpoints) {
	if duck.EndpointsAreAvailable(ep) {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionIngress)
	} else {
		bs.MarkIngressFailed("EndpointsUnavailable", "Endpoints %q are unavailable.", ep.Name)
	}
}

func MarkSecretFailed(bs *eventingv1.BrokerStatus, reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionSecret, reason, format, args...)
}

func MarkSecretReady(bs *eventingv1.BrokerStatus) {
	bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionSecret)
}

func MarkExchangeFailed(bs *eventingv1.BrokerStatus, reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionExchange, reason, format, args...)
}

func MarkExchangeReady(bs *eventingv1.BrokerStatus) {
	bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionExchange)
}

func MarkDLXFailed(bs *eventingv1.BrokerStatus, reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionDLX, reason, format, args...)
}

func MarkDLXReady(bs *eventingv1.BrokerStatus) {
	bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionDLX)
}

// SetAddress makes this Broker addressable by setting the URI. It also
// sets the BrokerConditionAddressable to true.
func SetAddress(bs *eventingv1.BrokerStatus, url *apis.URL) {
	bs.Address.URL = url
	if url != nil {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionAddressable)
	} else {
		bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionAddressable, "nil URL", "URL is nil")
	}
}
