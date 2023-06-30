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
	"bytes"
	"context"
	"testing"
	"time"

	sourcesv1alpha1 "knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	bindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	msgId           = "testuuid-123123123"
	namespace       = "testns"
	sourceName      = "test-source"
	queueName       = "test-queue"
	testContentType = "test-content-type"
)

var (
	msgTime = time.Now()
	source  = sourcesv1alpha1.RabbitmqEventSource(namespace, sourceName, queueName)
)

func TestProtocol_MsgStructured(t *testing.T) {
	for _, tt := range []struct {
		name    string
		headers map[string][]byte
		kind    spec.Kind
		version spec.Version
		want    interface{}
	}{{
		name:    "err not structured",
		headers: map[string][]byte{},
		kind:    spec.Kind(0),
		version: nil,
		want:    binding.ErrNotStructured,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			m := Message{
				Headers: tt.headers,
				version: tt.version,
			}

			mb := bindingtest.MockStructuredMessage{}
			err := m.ReadStructured(context.TODO(), &mb)
			if err != tt.want {
				t.Errorf("Unexpected error %s ", err)
			}
		})
	}
}

func TestProtocol_MsgReadBinary(t *testing.T) {
	for _, tt := range []struct {
		name    string
		headers map[string][]byte
		body    []byte
		version spec.Version
		wantErr error
	}{{
		name:    "err not binary",
		wantErr: binding.ErrNotBinary,
	}, {
		name:    "empty binary msg",
		version: specs.Version("1.0"),
	}, {
		name:    "normal binary msg",
		version: specs.Version("1.0"),
		headers: map[string][]byte{
			"content-type": []byte(testContentType),
			"id":           []byte(msgId),
			"ext":          []byte("test extension"),
		},
	}, {
		name:    "binary event with body",
		version: specs.Version("1.0"),
		body:    []byte("hey I'm a test!"),
		headers: map[string][]byte{
			"content-type": []byte(testContentType),
			"id":           []byte(msgId),
			"body":         []byte("Hey I'm a test body!"),
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			want := &Message{
				Headers:     tt.headers,
				version:     tt.version,
				ContentType: format.JSON.MediaType(),
				format:      format.JSON,
				Value:       tt.body,
			}

			mb := bindingtest.MockBinaryMessage{
				Metadata:   make(map[spec.Attribute]interface{}),
				Extensions: make(map[string]interface{}),
			}
			err := want.ReadBinary(context.TODO(), &mb)
			if err != tt.wantErr {
				t.Errorf("Unexpected error %s ", err)
			}

			mergedHeaders := make(map[string][]byte)
			for key, val := range mb.Extensions {
				mergedHeaders[key] = []byte(val.(string))
			}

			for key, val := range mb.Metadata {
				mergedHeaders[key.Name()] = []byte(val.(string))
			}

			got := &Message{
				Headers:     mergedHeaders,
				version:     tt.version,
				ContentType: format.JSON.MediaType(),
				format:      format.JSON,
				Value:       mb.Body,
			}
			if !compareMessages(want, got) {
				t.Errorf("Unexpected message want:\n%v\ngot:\n%v", want, got)
			}
		})
	}
}

func TestProtocol_MsgReadEncoding(t *testing.T) {
	for _, tt := range []struct {
		name    string
		format  format.Format
		version spec.Version
		want    binding.Encoding
	}{{
		name: "encoding unknown",
		want: binding.EncodingUnknown,
	}, {
		name:    "encoding binary",
		version: spec.V1,
		want:    binding.EncodingBinary,
	}, {
		name:   "encoding structured",
		format: format.JSON,
		want:   binding.EncodingStructured,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			m := Message{
				format:  tt.format,
				version: tt.version,
			}
			err := m.ReadEncoding()
			if err != tt.want {
				t.Errorf("Unexpected error %s ", err)
			}
		})
	}
}

func TestProtocol_NewMessage(t *testing.T) {
	for _, tt := range []struct {
		name, contentType string
		headers           map[string][]byte
		format            format.Format
		want              *Message
	}{{
		name:        "msg without format nor specversion",
		contentType: testContentType,
		want: &Message{
			ContentType: testContentType,
		},
	}, {
		name:        "msg with version",
		headers:     map[string][]byte{"specversion": []byte("1.0")},
		contentType: testContentType,
		want: &Message{
			ContentType: testContentType,
			version:     specs.Version("1.0"),
			Headers:     map[string][]byte{"specversion": []byte("1.0")},
		},
	}, {
		name:        "msg with format",
		format:      format.JSON,
		contentType: format.JSON.MediaType(),
		want: &Message{
			ContentType: format.JSON.MediaType(),
			format:      format.Lookup(format.JSON.MediaType()),
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			got := NewMessage([]byte{}, tt.contentType, tt.headers)
			if !compareMessages(tt.want, got) {
				t.Errorf("Unexpected message want:\n%v\ngot:\n%v", tt.want, got)
			}
		})
	}
}

func TestProtocol_MsgGetAttribute(t *testing.T) {
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

func TestProtocol_MsgGetExtension(t *testing.T) {
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
		headers: map[string][]byte{"extension": []byte("test")},
		extName: "extension",
		want:    "test",
	}, {
		name:    "get msg extension different value",
		headers: map[string][]byte{"different": []byte("testing this again 1")},
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

func TestProtocol_Finish(t *testing.T) {
	m := Message{}
	if err := m.Finish(nil); err != nil {
		t.Errorf("Unexpected msg finish return value want:\nnil\ngot:\n%v", err)
	}
}

func TestProtocol_ConvertToCloudEvent(t *testing.T) {
	for _, tt := range []struct {
		name     string
		delivery *amqp.Delivery
		err      error
	}{{
		name: "convert basic msg without id",
		delivery: &amqp.Delivery{
			Timestamp: msgTime,
		},
	}, {
		name: "convert basic msg with id",
		delivery: &amqp.Delivery{
			MessageId: msgId,
			Timestamp: msgTime,
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			event := cloudevents.NewEvent()
			got := ConvertToCloudEvent(&event, tt.delivery, namespace, sourceName, queueName)
			event.SetTime(msgTime)
			if got != tt.err {
				t.Errorf("Unexpected error converting msg want:\n%v\ngot:\n%v", tt.err, got)
			}

			want := cloudevents.NewEvent()
			if tt.delivery.MessageId != "" {
				want.SetID(tt.delivery.MessageId)
			} else {
				want.SetID(event.ID())
			}
			want.SetType(sourcesv1alpha1.RabbitmqEventType)
			want.SetSource(source)
			want.SetSubject(want.ID())
			want.SetTime(tt.delivery.Timestamp)
			if len(tt.delivery.Body) > 0 {
				want.SetData(tt.delivery.ContentType, tt.delivery.Body)
			}
			if event.String() != want.String() {
				t.Errorf("Unexpected event conversion want:\n%v\ngot:\n%v", want, event)
			}
		})
	}
}

func TestProtocol_NewMessageFromDelivery(t *testing.T) {
	for _, tt := range []struct {
		name     string
		headers  map[string][]byte
		delivery *amqp.Delivery
		want     *Message
	}{{
		name:    "set empty message",
		headers: map[string][]byte{},
		delivery: &amqp.Delivery{
			MessageId: msgId,
			Timestamp: msgTime,
		},
		want: &Message{Headers: make(map[string][]byte)},
	}, {
		name:    "set empty message",
		headers: map[string][]byte{},
		delivery: &amqp.Delivery{
			MessageId: msgId,
			Timestamp: msgTime,
		},
		want: &Message{Headers: make(map[string][]byte)},
	}, {
		name:    "set traceparent header",
		headers: map[string][]byte{"content-type": []byte(testContentType)},
		delivery: &amqp.Delivery{
			MessageId:   msgId,
			Timestamp:   msgTime,
			ContentType: testContentType,
			Headers:     amqp.Table{traceparent: "test-traceparent", tracestate: "test-tracestate"},
		},
		want: &Message{Headers: make(map[string][]byte), ContentType: testContentType},
	}, {
		name:    "set extension header",
		headers: map[string][]byte{"content-type": []byte(testContentType)},
		delivery: &amqp.Delivery{
			MessageId:   msgId,
			Timestamp:   msgTime,
			ContentType: testContentType,
			Headers:     amqp.Table{"ce-extension": "test extension"},
		},
		want: &Message{Headers: map[string][]byte{"ce-extension": []byte("test extension")}, ContentType: testContentType},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			got := NewMessageFromDelivery(sourceName, namespace, queueName, tt.delivery)
			if _, ok := tt.want.Headers["source"]; !ok {
				tt.want.Headers["source"] = []byte(source)
			}
			if !compareMessages(got, tt.want) {
				t.Errorf("Unexpected message want:\n%v\ngot:\n%v", tt.want, got)
			}
		})
	}
}

func TestProtocol_CloudEventToRabbitMQMessage(t *testing.T) {
	e := cloudevents.NewEvent()
	e.SetID("1234")
	e.SetType(sourcesv1alpha1.RabbitmqEventType)
	e.SetSource(sourcesv1alpha1.RabbitmqEventSource(namespace, sourceName, queueName))
	e.SetSubject(e.ID())
	e.SetTime(time.Now())
	e.SetExtension("testExtension", "testing the extensions")
	e.SetDataSchema("test.data.schema")
	e.SetData(*cloudevents.StringOfApplicationJSON(), map[string]string{"test": "test body data"})

	for _, tt := range []struct {
		name    string
		empty   bool
		wantMsg *amqp.Publishing
	}{{
		name:  "convert empty cloudevent",
		empty: true,
		wantMsg: &amqp.Publishing{
			DeliveryMode: 2,
			Timestamp:    time.Time{}.UTC(),
			Headers: amqp.Table{
				contentTypeHeader: "", "id": "", specversionHeader: "1.0", "time": cloudevents.Timestamp{Time: time.Time{}.UTC()}.String(),
			}},
	}, {
		name: "convert cloudevent",
		wantMsg: &amqp.Publishing{
			DeliveryMode: 2,
			Timestamp:    time.Time{}.UTC(),
			MessageId:    e.ID(),
			ContentType:  e.DataContentType(),
			Body:         e.Data(),
			Headers: amqp.Table{
				contentTypeHeader: e.DataContentType(),
				"id":              e.ID(),
				"subject":         e.Subject(),
				specversionHeader: e.SpecVersion(),
				"time":            cloudevents.Timestamp{Time: e.Time().UTC()}.String(),
				"source":          e.Source(),
				"type":            e.Type(),
				"testextension":   e.Extensions()["testextension"].(string),
				"dataschema":      e.DataSchema(),
				"traceparent":     "tp",
				"tracestate":      "ts",
			}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			testEvent := e
			tp := "tp"
			ts := "ts"
			if tt.empty {
				tp = ""
				ts = ""
				testEvent = cloudevents.NewEvent()
			}
			got := CloudEventToRabbitMQMessage(&testEvent, tp, ts)
			if !comparePublishings(tt.wantMsg, got) {
				t.Errorf("Unexpected RabbitMQ message want:\n%v\ngot:\n%v", tt.wantMsg, got)
			}
		})
	}
}

func compareMessages(m1, m2 *Message) bool {
	if len(m1.Headers) != len(m2.Headers) {
		return false
	}

	for key, val := range m1.Headers {
		if val2, ok := m2.Headers[key]; ok {
			if !bytes.Equal(val, val2) {
				return false
			}
		}
	}

	return (m1.format == m2.format && m1.version == m2.version &&
		m1.ContentType == m2.ContentType && bytes.Equal(m1.Value, m2.Value))
}

func comparePublishings(p1, p2 *amqp.Publishing) bool {
	if len(p1.Headers) != len(p2.Headers) {
		return false
	}

	for key, val := range p1.Headers {
		if val2, ok := p2.Headers[key]; ok {
			if val.(string) != val2.(string) {
				return false
			}
		}
	}

	return (bytes.Equal(p1.Body, p2.Body) && p1.MessageId == p2.MessageId &&
		p1.ContentType == p2.ContentType && p1.DeliveryMode == p2.DeliveryMode)
}
