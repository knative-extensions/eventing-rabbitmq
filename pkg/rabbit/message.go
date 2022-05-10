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
	"fmt"
	nethttp "net/http"
	"strings"

	"go.uber.org/zap"
	sourcesv1alpha1 "knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"

	"github.com/NeowayLabs/wabbit"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/protocol/http"

	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	prefix            = "ce-"
	contentTypeHeader = "content-type"
	specversionHeader = "specversion"
)

var specs = spec.WithPrefix(prefix)

// Message holds a rabbitmq message.
// this message *can* be read several times safely
type Message struct {
	Value       []byte
	Headers     map[string][]byte
	ContentType string
	format      format.Format
	version     spec.Version
}

// Check if rabbit.Message implements binding.Message
var _ binding.Message = (*Message)(nil)
var _ binding.MessageMetadataReader = (*Message)(nil)

func ConvertMessageToHTTPRequest(
	ctx context.Context,
	sourceName, namespace, queueName string,
	msg wabbit.Delivery,
	req *nethttp.Request,
	logger *zap.Logger) error {
	msgBinding := NewMessageFromDelivery(sourceName, namespace, queueName, msg)
	defer func() {
		err := msgBinding.Finish(nil)
		if err != nil {
			logger.Error("Something went wrong while trying to finalizing the message", zap.Error(err))
		}
	}()

	// if the msg is a cloudevent send it as it is to http
	if msgBinding.ReadEncoding() != binding.EncodingUnknown {
		return http.WriteRequest(cloudevents.WithEncodingBinary(ctx), msgBinding, req)
	}

	// if the rabbitmq msg is not a cloudevent transform it into one
	event := cloudevents.NewEvent()
	err := ConvertToCloudEvent(&event, msg, namespace, sourceName, queueName)
	if err != nil {
		logger.Error("Error converting RabbitMQ msg to CloudEvent", zap.Error(err))
		return err
	}

	return http.WriteRequest(ctx, binding.ToMessage(&event), req)
}

// NewMessageFromDelivery returns a binding.Message that holds the provided RabbitMQ Message.
// The returned binding.Message *can* be read several times safely
func NewMessageFromDelivery(sourceName, namespace, queueName string, msg wabbit.Delivery) *Message {
	headers := make(map[string][]byte, len(msg.Headers()))
	for key, val := range msg.Headers() {
		k := strings.ToLower(key)
		if k == contentTypeHeader {
			continue
		}
		headers[k] = []byte(fmt.Sprint(val))
	}

	if _, ok := headers["source"]; !ok {
		headers["source"] = []byte(sourcesv1alpha1.RabbitmqEventSource(namespace, sourceName, queueName))
	}

	return NewMessage(msg.Body(), msg.ContentType(), headers)
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
	} else if v := specs.Version(string(headers[specversionHeader])); v != nil {
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
		if k == contentTypeHeader {
			err = encoder.SetAttribute(m.version.AttributeFromKind(spec.DataContentType), string(v))
		} else {
			attr := m.version.Attribute(prefix + k)
			if attr != nil {
				err = encoder.SetAttribute(attr, string(v))
			} else {
				err = encoder.SetExtension(k, string(v))
			}
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
	return string(m.Headers[name])
}

func (m *Message) Finish(error) error {
	return nil
}

func ConvertToCloudEvent(event *cloudevents.Event, msg wabbit.Delivery, namespace, sourceName, queueName string) error {
	if msg.MessageId() != "" {
		event.SetID(msg.MessageId())
	} else {
		event.SetID(string(uuid.NewUUID()))
	}
	if event.Type() == "" {
		event.SetType(sourcesv1alpha1.RabbitmqEventType)
	}
	if event.Source() == "" {
		event.SetSource(sourcesv1alpha1.RabbitmqEventSource(namespace, sourceName, queueName))
	}

	event.SetSubject(event.ID())
	event.SetTime(msg.Timestamp())
	err := event.SetData(msg.ContentType(), msg.Body())
	if err != nil {
		return err
	}
	return nil
}
