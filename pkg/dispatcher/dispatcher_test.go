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

package dispatcher

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	"go.opencensus.io/trace"

	dispatcherstats "knative.dev/eventing-rabbitmq/pkg/broker/dispatcher"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
)

func TestDispatcher_ConsumeFromQueue(t *testing.T) {
	statsReporter := dispatcherstats.NewStatsReporter("test-container", "test-name", "test-ns")
	h := &fakeHandler{
		handlers: []handlerFunc{requestAccepted},
	}
	server := httptest.NewServer(h)
	defer server.Close()
	d := &Dispatcher{
		BackoffPolicy: v1.BackoffPolicyLinear,
		MaxRetries:    1,
		Timeout:       time.Duration(500),
		BackoffDelay:  time.Duration(500),
		WorkerCount:   10,
		Reporter:      statsReporter,
	}
	ctx, cancelFunc := context.WithCancel(context.TODO())
	go func() {
		time.Sleep(1000)
		cancelFunc()
	}()
	d.ConsumeFromQueue(ctx, &rabbit.RabbitMQChannelMock{}, "")
}

func TestDispatcher_ReadSpan(t *testing.T) {
	for _, tt := range []struct {
		name     string
		delivery amqp.Delivery
		err      bool
	}{
		{
			name:     "no traceparent",
			delivery: amqp.Delivery{},
			err:      true,
		}, {
			name:     "no traceparent",
			delivery: amqp.Delivery{Headers: amqp.Table{"traceparent": "tp-test"}},
			err:      true,
		}, {
			name:     "both trace headers set but no context",
			delivery: amqp.Delivery{Headers: amqp.Table{"traceparent": "tp-test", "tracestate": "ts-test"}},
			err:      true,
		}, {
			name: "valid span",
			err:  false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			ctx := context.TODO()
			d := tt.delivery
			if !tt.err {
				var span *trace.Span
				ctx, span = trace.StartSpanWithRemoteParent(ctx, "test-span", trace.SpanContext{})
				tp, ts := (&tracecontext.HTTPFormat{}).SpanContextToHeaders(span.SpanContext())
				d = amqp.Delivery{Headers: amqp.Table{"traceparent": tp, "tracestate": ts}}
			}

			ctx, span := readSpan(ctx, d)
			if span != nil && tt.err {
				t.Error("invalid context is returning a valid span")
			} else if span == nil && !tt.err {
				t.Error("valid span and context got an unexpected error")
			}
		})
	}
}

func TestDispatcher_getStatus(t *testing.T) {
	for _, tt := range []struct {
		name   string
		result protocol.Result
		err    bool
	}{
		{
			name:   "nil result",
			result: nil,
			err:    true,
		}, {
			name:   "invalid retry result",
			result: cehttp.NewRetriesResult(protocol.NewResult(""), 1, time.Now(), []protocol.Result{}),
			err:    true,
		}, {
			name:   "valid retry result",
			result: cehttp.NewRetriesResult(cehttp.NewResult(400, ""), 1, time.Now(), []protocol.Result{}),
			err:    false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			ctx := context.TODO()
			i, _ := getStatus(ctx, tt.result)
			if tt.err && i != -1 {
				t.Error("expecting not a retry result but got one")
			} else if !tt.err && i == -1 {
				t.Error("expecting a valid retry result but got none")
			}
		})
	}
}

type handlerFunc func(http.ResponseWriter, *http.Request)
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
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body

	defer r.Body.Close()
	h.receiveCount++
	h.handlers[h.receiveCount](w, r)
}

func requestAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}
