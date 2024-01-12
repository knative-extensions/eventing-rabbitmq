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

package rabbitmq

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	v2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing/pkg/adapter/v2"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	rectesting "knative.dev/pkg/reconciler/testing"

	// Fake injection client
	_ "knative.dev/pkg/client/injection/kube/client/fake"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		client                     MockClient
		reqBody, expectedEventType string
		reqHeaders                 http.Header
		data                       map[string]interface{}
		headers                    amqp.Table
		attributes                 map[string]string
		withMsgId, isCe, error     bool
		retry                      int
	}{
		"accepted": {
			client: MockClient{
				send: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
					return v2.NewHTTPRetriesResult(v2.NewHTTPResult(200, ""), 0, time.Now(), []protocol.Result{})
				},
			},
			reqBody: `{"key":"value"}`,
			data:    map[string]interface{}{"key": "value"},
		},
		"accepted with msg id": {
			client: MockClient{
				send: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
					return v2.NewHTTPRetriesResult(v2.NewHTTPResult(200, ""), 0, time.Now(), []protocol.Result{})
				},
			},
			reqBody:   `{"key":"value"}`,
			withMsgId: true,
			data:      map[string]interface{}{"key": "value"},
		},
		"accepted with binary cloudevent": {
			client: MockClient{
				send: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
					return v2.NewHTTPRetriesResult(v2.NewHTTPResult(200, ""), 0, time.Now(), []protocol.Result{})
				},
			},
			reqBody:   `{"test":"test"}`,
			withMsgId: true,
			reqHeaders: http.Header{
				"Specversion": []string{"1.0"},
				"Source":      []string{"example/source.uri"},
				"Testheader":  []string{"testHeader"},
			},
			data: map[string]interface{}{
				"test": "test",
			},
			headers: amqp.Table{
				"specversion": "1.0",
				"source":      "example/source.uri",
				"testheader":  "testHeader",
			},
			isCe: true,
		},
		"accepted with structured cloudevent": {
			client: MockClient{
				send: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
					return v2.NewHTTPRetriesResult(v2.NewHTTPResult(200, ""), 0, time.Now(), []protocol.Result{})
				},
			},
			reqBody: `{"specversion":"1.0","id":1234,` +
				`"type":"dev.knative.rabbitmq.event","source":"example/source.uri",` +
				`"content-type":"text/plain","data":"test"}`,
			withMsgId: true,
			headers:   amqp.Table{"content-type": "application/cloudevents+json"},
			data: map[string]interface{}{
				"specversion":  "1.0",
				"id":           1234,
				"type":         "dev.knative.rabbitmq.event",
				"source":       "example/source.uri",
				"content-type": "text/plain",
				"data":         "test",
			},
			isCe: true,
		},
		"rejected": {
			client: MockClient{
				send: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
					return v2.NewHTTPRetriesResult(v2.NewHTTPResult(500, ""), 0, time.Now(), []protocol.Result{amqp.Error{}})
				},
			},
			reqBody: `{"key":"value"}`,
			error:   true,
			data:    map[string]interface{}{"key": "value"},
		},
		"retried 3 times succesfull on the 4th ": {
			retry: 5,
			client: MockClient{
				send: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
					return v2.NewHTTPRetriesResult(v2.NewHTTPResult(200, ""), 3, time.Now(), []protocol.Result{amqp.Error{}, amqp.Error{}, amqp.Error{}})
				},
			},
			reqBody: `{"key":"value"}`,
			data:    map[string]interface{}{"key": "value"},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			client, err := v2.NewClient(tc.client)
			if err != nil {
				t.Fatalf("Failed to create protocol, %v", err)
			}

			config := adapterConfig{}
			if tc.retry > 0 {
				config = adapterConfig{Retry: tc.retry, BackoffPolicy: string(v1.BackoffPolicyLinear), BackoffDelay: "PT0.1S"}
			}
			a := &Adapter{
				config:  &config,
				context: context.TODO(),
				logger:  zap.NewNop(),
				client:  client,
			}

			data, err := json.Marshal(tc.data)
			if err != nil {
				t.Errorf("unexpected error, %v", err)
			}

			m := &amqp.Delivery{
				Body:    data,
				Headers: tc.headers,
			}

			if tc.withMsgId {
				m.MessageId = "id"
			}

			err = a.postMessage(m)
			if tc.error && err == nil {
				t.Errorf("expected error, but got %v", err)
			}
		})
	}
}

type MockClient struct {
	send func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error
}

func (mock MockClient) Send(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
	return mock.send(ctx, m, transformers...)
}

func TestAdapter_PollForMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &Adapter{
		config: &adapterConfig{
			ExchangeName:  "Test-exchange",
			QueueName:     "",
			Parallelism:   10,
			BackoffPolicy: string(v1.BackoffPolicyLinear),
			BackoffDelay:  "PT0.20S",
			Retry:         1,
		},
		context:   ctx,
		logger:    zap.NewNop(),
		rmqHelper: rabbit.NewRabbitMQConnectionHandler(5, 500, zap.NewNop().Sugar()),
	}
	a.rmqHelper.Setup(ctx, "", nil, rabbit.ValidDial)
	go func() {
		time.Sleep(500)
		// Signal to the adapter to finish and do not retry
		cancel()
	}()
	a.Start(a.context)
}

func TestAdapter_NewEnvConfig(t *testing.T) {
	env := NewEnvConfig()
	var envPlaceholder adapter.EnvConfigAccessor
	if reflect.TypeOf(env) == reflect.TypeOf(envPlaceholder) {
		t.Errorf("Error in NewnvConfig return Type")
	}
}

func TestAdapter_NewAdapter(t *testing.T) {
	ctx, _ := rectesting.SetupFakeContext(t)
	env := NewEnvConfig()
	client, err := v2.NewClient(MockClient{})
	if err != nil {
		t.Fatalf("Failed to create protocol, %v", err)
	}
	a := NewAdapter(ctx, env, nil)
	cmpA := &Adapter{
		config:  env.(*adapterConfig),
		logger:  logging.FromContext(ctx).Desugar(),
		context: ctx,
		client:  client,
	}

	if a == cmpA {
		t.Errorf("Error in NewnvConfig return Type")
	}
}
