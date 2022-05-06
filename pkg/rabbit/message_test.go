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

package rabbit

import (
	"context"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	bindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
)

func TestAdapter_MsgReadBinary(t *testing.T) {
	for _, tt := range []struct {
		name    string
		headers map[string][]byte
		kind    spec.Kind
		version spec.Version
		want    interface{}
	}{{
		name:    "err not binary",
		headers: map[string][]byte{},
		kind:    spec.Kind(0),
		version: nil,
		want:    binding.ErrNotBinary,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			m := Message{
				Headers: tt.headers,
				version: tt.version,
			}

			mb := bindingtest.MockBinaryMessage{}
			err := m.ReadBinary(context.TODO(), &mb)
			if err != tt.want {
				t.Errorf("Unexpected error %s ", err)
			}
		})
	}
}

func TestAdapter_MsgGetAttribute(t *testing.T) {
	for _, tt := range []struct {
		name    string
		headers map[string][]byte
		kind    spec.Kind
		want    interface{}
	}{{
		name:    "get empty attribute",
		headers: map[string][]byte{},
		kind:    spec.Kind(0),
		want:    "",
	}, {
		name:    "get msg id from kind",
		headers: map[string][]byte{"id": []byte("1234")},
		kind:    spec.Kind(0),
		want:    "1234",
	}, {
		name:    "get non existent attribute",
		headers: map[string][]byte{"does not exist": []byte("test")},
		kind:    spec.Kind(16),
		want:    nil,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			m := Message{
				Headers: tt.headers,
				version: spec.V1,
			}

			attr, got := m.GetAttribute(tt.kind)
			if got != tt.want {
				t.Errorf("Unexpected attribute value %s want:\n%v\ngot:\n%v", attr, tt.want, got)
			}
		})
	}
}

func TestAdapter_MsgGetExtension(t *testing.T) {
	for _, tt := range []struct {
		name    string
		headers map[string][]byte
		extName string
		want    string
	}{{
		name:    "get empty extension",
		headers: map[string][]byte{},
		extName: "invalid",
		want:    "",
	}, {
		name:    "get msg extension",
		headers: map[string][]byte{"ce-extension": []byte("test")},
		extName: "extension",
		want:    "test",
	}, {
		name:    "get msg extension different value",
		headers: map[string][]byte{"ce-different": []byte("testing this again 1")},
		extName: "different",
		want:    "testing this again 1",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			m := Message{
				Headers: tt.headers,
			}

			got := m.GetExtension(tt.extName)
			if got != tt.want {
				t.Errorf("Unexpected extension value %s want:\n%v\ngot:\n%v", tt.extName, tt.want, got)
			}
		})
	}
}
