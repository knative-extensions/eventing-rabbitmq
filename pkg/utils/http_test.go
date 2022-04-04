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
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"testing"
	"time"
)

func TestWithTimeout(t *testing.T) {
	newProtocol, err := cehttp.New([]cehttp.Option{}...)
	if err != nil {
		t.Fatal("couldn't create new Protocol")
	}

	testCases := map[string]struct{
		protocol *cehttp.Protocol
		timeout time.Duration
		expectErr bool
	}{
		"should error if protocol is nil": {
			protocol: nil,
			timeout: 30 * time.Second,
			expectErr: true,
		},
		"should initialize client if none exists": {
			protocol: &cehttp.Protocol{},
			timeout: 30 * time.Second,
			expectErr: false,
		},
		"should set timeout on an initialized protocol": {
			protocol: newProtocol,
			timeout: 30 * time.Second,
			expectErr: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := WithTimeout(tc.timeout)(tc.protocol)
			if tc.expectErr && err == nil {
				t.Fatal("expected error but didn't receive one.")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("expected no error but received: %s", err)
			}

			if tc.protocol != nil {
				if tc.protocol.Client == nil {
					t.Fatal("client wasn't initialized")
				}

				if tc.protocol.Client.Timeout != tc.timeout {
					t.Fatalf("expected timeout: %s, got timeout: %s", tc.timeout, tc.protocol.Client.Timeout)
				}
			}
		})
	}
}
