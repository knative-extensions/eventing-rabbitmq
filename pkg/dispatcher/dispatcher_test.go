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
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	v2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"

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

	if err := d.ConsumeFromQueue(ctx, &rabbit.RabbitMQConnectionMock{}, &rabbit.RabbitMQChannelMock{}, ""); err != nil {
		t.Errorf("ConsumeFromQueue() error = %v", err)
	}
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

			_, span := readSpan(ctx, d)
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

func TestDispatcher_finishConsuming(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	workerQueue := make(chan amqp.Delivery)
	go func() {
		time.Sleep(time.Millisecond * 100)
		finishConsuming(wg, workerQueue)
	}()
	wg.Done()
	if _, ok := <-workerQueue; ok {
		t.Error("channel should be closed by the finishConsuming function")
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

func requestAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

type MockAcknowledger struct {
}

func (m MockAcknowledger) Ack(tag uint64, multiple bool) error {
	return nil
}
func (m MockAcknowledger) Nack(tag uint64, multiple bool, requeue bool) error {
	return nil
}
func (m MockAcknowledger) Reject(tag uint64, requeue bool) error {
	return nil
}

type MockClient struct {
	request func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (binding.Message, error)
}

func (mock MockClient) Request(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (binding.Message, error) {
	return mock.request(ctx, m, transformers...)
}

type MockStatsReporter struct {
}

func (m MockStatsReporter) ReportEventCount(args *dispatcherstats.ReportArgs, responseCode int) error {
	return nil
}

func (m MockStatsReporter) ReportEventDispatchTime(args *dispatcherstats.ReportArgs, responseCode int, d time.Duration) error {
	return nil
}

func TestDispatcher_dispatch(t *testing.T) {
	channel := rabbit.RabbitMQChannelMock{}

	type fields struct {
		BrokerIngressURL  string
		SubscriberURL     string
		SubscriberCACerts string
		MaxRetries        int
		BackoffDelay      time.Duration
		Timeout           time.Duration
		BackoffPolicy     v1.BackoffPolicyType
		WorkerCount       int
		Reporter          dispatcherstats.StatsReporter
		DLX               bool
	}
	type args struct {
		ctx     context.Context
		msg     amqp.Delivery
		client  MockClient
		channel rabbit.RabbitMQChannelInterface
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "invalid event",
			fields: fields{},
			args: args{
				ctx: context.TODO(),
				msg: amqp.Delivery{
					Acknowledger: &MockAcknowledger{},
					ContentType:  "application/cloudevents+json",
					Headers:      amqp.Table{},
				},
				client:  MockClient{},
				channel: nil,
			},
			wantErr: true,
		},
		{
			name: "invalid request",
			fields: fields{
				Reporter: &MockStatsReporter{},
			},
			args: args{
				ctx: context.TODO(),
				msg: amqp.Delivery{
					Acknowledger: &MockAcknowledger{},
					ContentType:  "application/cloudevents+json",
					Headers:      amqp.Table{},
					Body:         []byte(`{"specversion":"1.0","source":"valid-event","id":"valid-id","type":"valid-type"}`),
				},
				client: MockClient{
					request: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (binding.Message, error) {
						return m, v2.NewHTTPRetriesResult(v2.NewHTTPResult(500, ""), 0, time.Now(), []protocol.Result{})
					},
				},
				channel: &channel,
			},
			wantErr: true,
		},
		{
			name: "invalid request dlq",
			fields: fields{
				Reporter: &MockStatsReporter{},
				DLX:      true,
			},
			args: args{
				ctx: context.TODO(),
				msg: amqp.Delivery{
					Acknowledger: &MockAcknowledger{},
					ContentType:  "application/cloudevents+json",
					Headers:      amqp.Table{},
					Body:         []byte(`{"specversion":"1.0","source":"valid-event","id":"valid-id","type":"valid-type"}`),
				},
				client: MockClient{
					request: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (binding.Message, error) {
						return m, v2.NewHTTPRetriesResult(v2.NewHTTPResult(500, ""), 0, time.Now(), []protocol.Result{})
					},
				},
				channel: &channel,
			},
			wantErr: true,
		},
		{
			name: "valid event",
			fields: fields{
				Reporter: &MockStatsReporter{},
			},
			args: args{
				ctx: context.TODO(),
				msg: amqp.Delivery{
					Acknowledger: &MockAcknowledger{},
					ContentType:  "application/cloudevents+json",
					Headers:      amqp.Table{},
					Body:         []byte(`{"specversion":"1.0","source":"valid-event","id":"valid-id","type":"valid-type"}`),
				},
				client: MockClient{
					request: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (binding.Message, error) {
						return m, v2.NewHTTPRetriesResult(v2.NewHTTPResult(200, ""), 0, time.Now(), []protocol.Result{})
					},
				},
				channel: &channel,
			},
		},
		{
			name: "valid event dlq",
			fields: fields{
				Reporter: &MockStatsReporter{},
				DLX:      true,
			},
			args: args{
				ctx: context.TODO(),
				msg: amqp.Delivery{
					Acknowledger: &MockAcknowledger{},
					ContentType:  "application/cloudevents+json",
					Headers:      amqp.Table{},
					Body:         []byte(`{"specversion":"1.0","source":"valid-event","id":"valid-id","type":"valid-type"}`),
				},
				client: MockClient{
					request: func(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (binding.Message, error) {
						return m, v2.NewHTTPRetriesResult(v2.NewHTTPResult(200, ""), 0, time.Now(), []protocol.Result{})
					},
				},
				channel: &channel,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dispatcher{
				BrokerIngressURL:  tt.fields.BrokerIngressURL,
				SubscriberURL:     tt.fields.SubscriberURL,
				SubscriberCACerts: tt.fields.SubscriberCACerts,
				MaxRetries:        tt.fields.MaxRetries,
				BackoffDelay:      tt.fields.BackoffDelay,
				Timeout:           tt.fields.Timeout,
				BackoffPolicy:     tt.fields.BackoffPolicy,
				WorkerCount:       tt.fields.WorkerCount,
				Reporter:          tt.fields.Reporter,
				DLX:               tt.fields.DLX,
			}

			client, err := v2.NewClient(tt.args.client)
			if err != nil {
				log.Fatalf("Failed to create protocol, %v", err)
			}

			if d.DLX {
				if err = d.dispatchDLQ(tt.args.ctx, tt.args.msg, client); (err != nil) != tt.wantErr {
					t.Errorf("dispatchDLQ() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				if err = d.dispatch(tt.args.ctx, tt.args.msg, client, tt.args.channel); (err != nil) != tt.wantErr {
					t.Errorf("dispatch() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
