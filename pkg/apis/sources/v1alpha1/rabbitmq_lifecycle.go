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

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	"knative.dev/eventing/pkg/apis/duck"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	RabbitmqConditionReady                           = apis.ConditionReady
	RabbitmqConditionSinkProvided apis.ConditionType = "SinkProvided"
	RabbitmqConditionDeployed     apis.ConditionType = "Deployed"
	RabbitmqConditionExchange     apis.ConditionType = "ExchangeReady"
	RabbitmqConditionSecret       apis.ConditionType = "SecretReady"
	RabbitmqConditionResources    apis.ConditionType = "ResourcesReady"
)

var RabbitmqSourceCondSet = apis.NewLivingConditionSet(
	RabbitmqConditionSinkProvided,
	RabbitmqConditionDeployed,
	RabbitmqConditionExchange,
	RabbitmqConditionSecret)

func (s *RabbitmqSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return RabbitmqSourceCondSet.Manage(s).GetCondition(t)
}

func (s *RabbitmqSourceStatus) GetTopLevelCondition() *apis.Condition {
	return RabbitmqSourceCondSet.Manage(s).GetTopLevelCondition()
}

func (in *RabbitmqSource) GetStatus() *duckv1.Status {
	return &in.Status.Status
}

func (in *RabbitmqSource) GetConditionSet() apis.ConditionSet {
	return RabbitmqSourceCondSet
}

func (s *RabbitmqSourceStatus) IsReady() bool {
	return RabbitmqSourceCondSet.Manage(s).IsHappy()
}

func (s *RabbitmqSourceStatus) InitializeConditions() {
	RabbitmqSourceCondSet.Manage(s).InitializeConditions()
}

func (s *RabbitmqSourceStatus) MarkSink(uri *apis.URL) {
	s.SinkURI = uri
	if !uri.IsEmpty() {
		RabbitmqSourceCondSet.Manage(s).MarkTrue(RabbitmqConditionSinkProvided)
	} else {
		RabbitmqSourceCondSet.Manage(s).MarkUnknown(RabbitmqConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty")
	}
}

func (s *RabbitmqSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionSinkProvided, reason, messageFormat, messageA...)
}

func DeploymentIsAvailable(d *appsv1.DeploymentStatus, def bool) bool {
	for _, cond := range d.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			return cond.Status == "True"
		}
	}
	return def
}

func (s *RabbitmqSourceStatus) MarkDeployed(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		RabbitmqSourceCondSet.Manage(s).MarkTrue(RabbitmqConditionDeployed)
	} else {
		RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionDeployed, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

func (s *RabbitmqSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	RabbitmqSourceCondSet.Manage(s).MarkUnknown(RabbitmqConditionDeployed, reason, messageFormat, messageA...)
}

func (s *RabbitmqSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionDeployed, reason, messageFormat, messageA...)
}

func (s *RabbitmqSourceStatus) MarkResourcesCorrect() {
	RabbitmqSourceCondSet.Manage(s).MarkTrue(RabbitmqConditionResources)
}

func (s *RabbitmqSourceStatus) MarkResourcesIncorrect(reason, messageFormat string, messageA ...interface{}) {
	RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionResources, reason, messageFormat, messageA...)
}

func (s *RabbitmqSourceStatus) MarkExchangeFailed(reason, format string, args ...interface{}) {
	RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionExchange, reason, format, args...)
}

func (s *RabbitmqSourceStatus) MarkExchangeReady() {
	RabbitmqSourceCondSet.Manage(s).MarkTrue(RabbitmqConditionExchange)
}

func (s *RabbitmqSourceStatus) MarkSecretFailed(reason, format string, args ...interface{}) {
	RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionSecret, reason, format, args...)
}

func (s *RabbitmqSourceStatus) MarkSecretReady() {
	RabbitmqSourceCondSet.Manage(s).MarkTrue(RabbitmqConditionSecret)
}
