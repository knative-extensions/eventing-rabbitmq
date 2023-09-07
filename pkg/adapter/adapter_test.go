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
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing/pkg/adapter/v2"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/metrics/source"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
)

var serverTestName = "test-name"

type handlerFunc func(http.ResponseWriter, *http.Request)

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		handlers                   []handlerFunc
		reqBody, expectedEventType string
		reqHeaders                 http.Header
		data                       map[string]interface{}
		headers                    amqp.Table
		attributes                 map[string]string
		withMsgId, isCe, error     bool
		retry                      int
	}{
		"accepted": {
			handlers: []handlerFunc{sinkAccepted},
			reqBody:  `{"key":"value"}`,
			data:     map[string]interface{}{"key": "value"},
		},
		"accepted with msg id": {
			handlers:  []handlerFunc{sinkAccepted},
			reqBody:   `{"key":"value"}`,
			withMsgId: true,
			data:      map[string]interface{}{"key": "value"},
		},
		"accepted with binary cloudevent": {
			handlers:  []handlerFunc{sinkAccepted},
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
			handlers: []handlerFunc{sinkAccepted},
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
			handlers: []handlerFunc{sinkRejected},
			reqBody:  `{"key":"value"}`,
			error:    true,
			data:     map[string]interface{}{"key": "value"},
		},
		"retried 3 times succesfull on the 4th ": {
			retry:    5,
			handlers: []handlerFunc{sinkRejected, sinkRejected, sinkRejected, sinkAccepted},
			reqBody:  `{"key":"value"}`,
			error:    false,
			data:     map[string]interface{}{"key": "value"},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handlers: tc.handlers,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			target, err := apis.ParseURL(sinkServer.URL)
			if err != nil {
				t.Fatal(err)
			}
			sink := duckv1.Addressable{
				Name: &serverTestName,
				URL:  target,
			}
			statsReporter, _ := source.NewStatsReporter()
			config := adapterConfig{}
			if tc.retry > 0 {
				config = adapterConfig{Retry: tc.retry, BackoffPolicy: string(v1.BackoffPolicyLinear), BackoffDelay: "PT0.1S"}
			}
			a := &Adapter{
				config:   &config,
				context:  context.TODO(),
				sink:     sink,
				logger:   zap.NewNop(),
				reporter: statsReporter,
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

			var wantBody, gotBody map[string]interface{}
			err = json.Unmarshal([]byte(tc.reqBody), &wantBody)
			if err != nil {
				t.Errorf("Error unmarshaling wanted request body %s %s", tc.reqBody, err)
			}
			err = json.Unmarshal(h.body, &gotBody)
			if err != nil {
				t.Errorf("Error unmarshaling got request body %s %s", h.body, err)
			}

			if len(wantBody) > 0 && len(wantBody) != len(gotBody) && !reflect.DeepEqual(wantBody, gotBody) {
				t.Errorf("Expected request body '%s', but got '%s' %s", tc.reqBody, h.body, err)
			}

			if tc.isCe {
				ceHeaders := http.Header{}
				for key, value := range h.header {
					ceHeaders[strings.TrimPrefix(key, "Ce-")] = value
				}

				if !compareHeaders(tc.reqHeaders, ceHeaders, t) {
					t.Errorf("Expected request headers '%s', but got '%s' %s", tc.reqHeaders, ceHeaders, err)
				}
			}
		})
	}
}

func compareHeaders(expected, received http.Header, t *testing.T) bool {
	for key, val := range expected {
		if val2, ok := received[key]; !ok || val[0] != val2[0] {
			return false
		}
	}
	return true
}

type fakeHandler struct {
	body   []byte
	header http.Header
	mu     sync.Mutex

	receiveCount int
	handlers     []handlerFunc
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.header = r.Header
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body

	defer r.Body.Close()
	h.receiveCount++
	h.handlers[h.receiveCount](w, r)
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
}

func TestAdapter_PollForMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	statsReporter, _ := source.NewStatsReporter()
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
		reporter:  statsReporter,
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
	ctx := context.TODO()
	env := NewEnvConfig()
	h := &fakeHandler{
		handlers: []handlerFunc{sinkAccepted},
	}

	sinkServer := httptest.NewServer(h)
	defer sinkServer.Close()

	target, err := apis.ParseURL(sinkServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	sink := duckv1.Addressable{
		Name: &serverTestName,
		URL:  target,
	}
	statsReporter, _ := source.NewStatsReporter()
	a := NewAdapter(ctx, env, sink, statsReporter)
	cmpA := &Adapter{
		config:   env.(*adapterConfig),
		sink:     sink,
		reporter: statsReporter,
		logger:   logging.FromContext(ctx).Desugar(),
		context:  ctx,
	}

	if a == cmpA {
		t.Errorf("Error in NewnvConfig return Type")
	}
}
