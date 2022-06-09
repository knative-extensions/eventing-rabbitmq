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

package utils

import (
	"context"
	"testing"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

func TestSetBackoffPolicy(t *testing.T) {
	testCases := map[string]struct {
		policyString   string
		expectedPolicy eventingduckv1.BackoffPolicyType
	}{
		"should return exponential policy when policy string is empty": {
			policyString:   "",
			expectedPolicy: eventingduckv1.BackoffPolicyExponential,
		},
		"should return exponential policy when policy string is exponential": {
			policyString:   "exponential",
			expectedPolicy: eventingduckv1.BackoffPolicyExponential,
		},
		"should return linear policy when policy string is linear": {
			policyString:   "linear",
			expectedPolicy: eventingduckv1.BackoffPolicyLinear,
		},
		"should return empty policy when policy string is not linear nor exponential": {
			policyString:   "unknownPolicy",
			expectedPolicy: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc := tc
			t.Parallel()
			policy := SetBackoffPolicy(context.TODO(), tc.policyString)
			if policy != tc.expectedPolicy {
				t.Errorf("Unexpected policy returned: want:\n%+s\ngot:\n%+s\n", tc.expectedPolicy, policy)
			}
		})
	}
}
