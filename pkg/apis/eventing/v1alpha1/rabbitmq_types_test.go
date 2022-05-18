/*
Copyright 2022 The Knative Authors

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
	"knative.dev/eventing/pkg/apis/eventing"
	"testing"
)

func TestRabbitmqBrokerConfig_GetGroupVersionKind(t *testing.T) {
	r := RabbitmqBrokerConfig{}
	gvk := r.GetGroupVersionKind()

	if gvk.Kind != "RabbitmqBrokerConfig" {
		t.Error("kind should be RabbitmqBrokerConfig")
	}
	if gvk.Version != "v1alpha1" {
		t.Error("version should be v1alpha1")
	}
	if gvk.Group != eventing.GroupName {
		t.Errorf("group name should be: %s", eventing.GroupName)
	}
}
