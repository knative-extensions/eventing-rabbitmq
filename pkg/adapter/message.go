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

package rabbitmq

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/NeowayLabs/wabbit"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
)

const (
	prefix            = "ce-"
	contentTypeHeader = "content-type"
)

var (
	specs = spec.WithPrefix(prefix)
)

// Message holds a rabbitmq message.
// this message *can* be read several times safely
type Message struct {
	Value       []byte
	Headers     map[string][]byte
	ContentType string
	format      format.Format
	version     spec.Version
}

// Check if http.Message implements binding.Message
var _ binding.Message = (*Message)(nil)
var _ binding.MessageMetadataReader = (*Message)(nil)

// NewMessageFromDelivery returns a binding.Message that holds the provided RabbitMQ Message.
// The returned binding.Message *can* be read several times safely
func NewMessageFromDelivery(msg wabbit.Delivery) *Message {
	var contentType string
	headers := make(map[string][]byte, len(msg.Headers()))
	for key, val := range msg.Headers() {
		k := strings.ToLower(key)
		if k == contentTypeHeader {
			contentType = val.(string)
		}

		headers[k] = []byte(fmt.Sprint(val))
	}

	return NewMessage(msg.Body(), contentType, headers)
}

// NewMessage returns a binding.Message that holds the provided rabbitmq message components.
// The returned binding.Message *can* be read several times safely
func NewMessage(value []byte, contentType string, headers map[string][]byte) *Message {
	if ft := format.Lookup(contentType); ft != nil {
		return &Message{
			Value:       value,
			ContentType: contentType,
			Headers:     headers,
			format:      ft,
		}
	} else if v := specs.Version(string(headers[specs.PrefixedSpecVersionName()])); v != nil {
		return &Message{
			Value:       value,
			ContentType: contentType,
			Headers:     headers,
			version:     v,
		}
	}

	return &Message{
		Value:       value,
		ContentType: contentType,
		Headers:     headers,
	}
}

func (m *Message) ReadEncoding() binding.Encoding {
	if m.version != nil {
		return binding.EncodingBinary
	}

	if m.format != nil {
		return binding.EncodingStructured
	}

	return binding.EncodingUnknown
}

func (m *Message) ReadBinary(ctx context.Context, encoder binding.BinaryWriter) (err error) {
	if m.version == nil {
		return binding.ErrNotBinary
	}

	for k, v := range m.Headers {
		if strings.HasPrefix(k, prefix) {
			attr := m.version.Attribute(k)
			if attr != nil {
				err = encoder.SetAttribute(attr, string(v))
			} else {
				err = encoder.SetExtension(strings.TrimPrefix(k, prefix), string(v))
			}
		} else if k == contentTypeHeader {
			err = encoder.SetAttribute(m.version.AttributeFromKind(spec.DataContentType), string(v))
		}

		if err != nil {
			return
		}
	}

	if m.Value != nil {
		err = encoder.SetData(bytes.NewBuffer(m.Value))
	}

	return
}

func (m *Message) ReadStructured(ctx context.Context, encoder binding.StructuredWriter) error {
	if m.format != nil {
		return encoder.SetStructuredEvent(ctx, m.format, bytes.NewReader(m.Value))
	}

	return binding.ErrNotStructured
}

func (m *Message) GetAttribute(k spec.Kind) (spec.Attribute, interface{}) {
	attr := m.version.AttributeFromKind(k)
	if attr != nil {
		return attr, string(m.Headers[attr.PrefixedName()])
	}

	return nil, nil
}

func (m *Message) GetExtension(name string) interface{} {
	return string(m.Headers[prefix+name])
}

func (m *Message) Finish(error) error {
	return nil
}
