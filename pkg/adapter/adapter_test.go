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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/NeowayLabs/wabbit"
	"github.com/NeowayLabs/wabbit/amqp"
	"github.com/NeowayLabs/wabbit/amqptest"
	"github.com/NeowayLabs/wabbit/amqptest/server"
	origamqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/metrics/source"
	"knative.dev/pkg/logging"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink                       func(http.ResponseWriter, *http.Request)
		reqBody, expectedEventType string
		reqHeaders                 http.Header
		data                       map[string]interface{}
		headers                    wabbit.Option
		attributes                 map[string]string
		withMsgId, isCe, error     bool
	}{
		"accepted": {
			sink:    sinkAccepted,
			reqBody: `{"key":"value"}`,
			data:    map[string]interface{}{"key": "value"},
		},
		"accepted with msg id": {
			sink:      sinkAccepted,
			reqBody:   `{"key":"value"}`,
			withMsgId: true,
			data:      map[string]interface{}{"key": "value"},
		},
		"accepted with binary cloudevent": {
			sink:      sinkAccepted,
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
			headers: wabbit.Option{
				"specversion": "1.0",
				"source":      "example/source.uri",
				"testheader":  "testHeader",
			},
			isCe: true,
		},
		"accepted with structured cloudevent": {
			sink: sinkAccepted,
			reqBody: `{"specversion":"1.0","id":1234,` +
				`"type":"dev.knative.rabbitmq.event","source":"example/source.uri",` +
				`"content-type":"text/plain","data":"test"}`,
			withMsgId: true,
			headers:   wabbit.Option{"content-type": "application/cloudevents+json"},
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
			sink:    sinkRejected,
			reqBody: `{"key":"value"}`,
			error:   true,
			data:    map[string]interface{}{"key": "value"},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			s, err := kncloudevents.NewHTTPMessageSenderWithTarget(sinkServer.URL)
			if err != nil {
				t.Fatal(err)
			}

			statsReporter, _ := source.NewStatsReporter()

			a := &Adapter{
				config: &adapterConfig{
					Brokers: "amqp://guest:guest@localhost:5672/",
					ExchangeConfig: ExchangeConfig{
						Type:       "topic",
						Durable:    true,
						AutoDelete: false,
					},
					QueueConfig: QueueConfig{
						Name:       "",
						Durable:    false,
						AutoDelete: false,
					},
				},
				context:           context.TODO(),
				httpMessageSender: s,
				logger:            zap.NewNop(),
				reporter:          statsReporter,
			}

			data, err := json.Marshal(tc.data)
			if err != nil {
				t.Errorf("unexpected error, %v", err)
			}

			m := &amqp.Delivery{}
			m.Delivery = &origamqp.Delivery{
				Body:    data,
				Headers: origamqp.Table(tc.headers),
			}

			if tc.withMsgId {
				m.Delivery.MessageId = "id"
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

func TestAdapter_CreateConn(t *testing.T) {
	fakeServer := server.NewServer("amqp://localhost:5672/%2f")
	err := fakeServer.Start()
	if err != nil {
		t.Errorf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	statsReporter, _ := source.NewStatsReporter()

	a := &Adapter{
		config: &adapterConfig{
			Brokers: "amqp://localhost:5672/%2f",
			ExchangeConfig: ExchangeConfig{
				Type:       "direct",
				Durable:    true,
				AutoDelete: false,
			},
			QueueConfig: QueueConfig{
				Name:       "",
				Durable:    false,
				AutoDelete: false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}

	conn, _ := a.CreateConn("", "", logging.FromContext(context.TODO()).Desugar())
	if conn.(*amqp.Conn) != nil {
		t.Errorf("Failed to connect to RabbitMQ")
	}

	conn, _ = a.CreateConn("guest", "guest", logging.FromContext(context.TODO()).Desugar())
	if conn.(*amqp.Conn) != nil {
		t.Errorf("Failed to connect to RabbitMQ")
	}
	fakeServer.Stop()
}

func TestAdapter_CreateChannel(t *testing.T) {
	fakeServer := server.NewServer("amqp://localhost:5672/%2f")
	err := fakeServer.Start()
	if err != nil {
		t.Errorf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	conn, err := amqptest.Dial("amqp://localhost:5672/%2f")
	if err != nil {
		t.Errorf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	statsReporter, _ := source.NewStatsReporter()

	a := &Adapter{
		config: &adapterConfig{
			Brokers: "amqp://localhost:5672/%2f",
			ExchangeConfig: ExchangeConfig{
				Type:       "direct",
				Durable:    true,
				AutoDelete: false,
			},
			QueueConfig: QueueConfig{
				Name:       "",
				Durable:    false,
				AutoDelete: false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}

	for i := 1; i <= 2048; i++ {
		channel, _ := a.CreateChannel(nil, conn, logging.FromContext(context.TODO()).Desugar())
		if channel == nil {
			t.Logf("Failed to open a channel")
			break
		}
	}
	fakeServer.Stop()
}

func TestAdapter_StartAmqpClient(t *testing.T) {
	testQueue := "testqueue"
	fakeServer := server.NewServer("amqp://localhost:5674/%2f")
	err := fakeServer.Start()
	if err != nil {
		t.Errorf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	conn, err := amqptest.Dial("amqp://localhost:5674/%2f")
	if err != nil {
		t.Errorf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	statsReporter, _ := source.NewStatsReporter()

	channel, err := conn.Channel()
	if err != nil {
		t.Errorf("Failed to open a channel: %s", err)
	}

	a := &Adapter{
		config: &adapterConfig{
			Brokers:     "amqp://localhost:5674/%2f",
			Predeclared: true,
			QueueConfig: QueueConfig{
				Name:       testQueue,
				Durable:    false,
				AutoDelete: false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}

	_, err = a.StartAmqpClient(&channel)
	if err.Error() != fmt.Sprintf("no queue named %s", testQueue) {
		t.Errorf("Was expecting an error due to invalid queue name, got error: %s", err)
	}

	_, err = channel.QueueDeclare(testQueue, wabbit.Option{})
	if err != nil {
		t.Errorf("Failed to declare new Queue: %s", err)
	}
	queue, err := a.StartAmqpClient(&channel)
	if err != nil {
		t.Errorf("An error has occurred and it shouldn't: %s", err)
	}

	_, err = a.ConsumeMessages(&channel, queue, logging.FromContext(context.TODO()).Desugar())
	if err != nil {
		t.Errorf("Failed to consume from RabbitMQ: %s", err)
	}
	fakeServer.Stop()
}

type fakeHandler struct {
	body   []byte
	header http.Header

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.header = r.Header
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body

	defer r.Body.Close()
	h.handler(w, r)
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
}

func TestAdapter_VhostHandler(t *testing.T) {
	for _, tt := range []struct {
		name    string
		brokers string
		vhost   string
		want    string
	}{{
		name:    "no brokers nor vhost",
		brokers: "",
		vhost:   "",
		want:    "",
	}, {
		name:    "no vhost",
		brokers: "amqp://localhost:5672",
		vhost:   "",
		want:    "amqp://localhost:5672",
	}, {
		name:    "no brokers",
		brokers: "",
		vhost:   "test-vhost",
		want:    "test-vhost",
	}, {
		name:    "brokers and vhost without separating slash",
		brokers: "amqp://localhost:5672",
		vhost:   "test-vhost",
		want:    "amqp://localhost:5672/test-vhost",
	}, {
		name:    "brokers and vhost without separating slash but vhost with ending slash",
		brokers: "amqp://localhost:5672",
		vhost:   "test-vhost/",
		want:    "amqp://localhost:5672/test-vhost/",
	}, {
		name:    "brokers with trailing slash and vhost without the slash",
		brokers: "amqp://localhost:5672/",
		vhost:   "test-vhost",
		want:    "amqp://localhost:5672/test-vhost",
	}, {
		name:    "vhost starting with slash and brokers without the slash",
		brokers: "amqp://localhost:5672",
		vhost:   "/test-vhost",
		want:    "amqp://localhost:5672/test-vhost",
	}, {
		name:    "brokers and vhost with slash",
		brokers: "amqp://localhost:5672/",
		vhost:   "/test-vhost",
		want:    "amqp://localhost:5672//test-vhost",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			got := vhostHandler(tt.brokers, tt.vhost)
			if got != tt.want {
				t.Errorf("Unexpected URI for %s/%s want:\n%+s\ngot:\n%+s", tt.brokers, tt.vhost, tt.want, got)
			}
		})
	}
}

func TestAdapter_PollForMessages(t *testing.T) {
	fakeServer := server.NewServer("amqp://localhost:5672/%2f")
	err := fakeServer.Start()
	if err != nil {
		t.Errorf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	conn, err := amqptest.Dial("amqp://localhost:5672/%2f")
	if err != nil {
		t.Errorf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		t.Errorf("Failed to open a channel")
	}

	statsReporter, _ := source.NewStatsReporter()

	a := &Adapter{
		config: &adapterConfig{
			Brokers: "amqp://guest:guest@localhost:5672/",
			ExchangeConfig: ExchangeConfig{
				Name:       "Test-exchange",
				Type:       "topic",
				Durable:    true,
				AutoDelete: false,
			},
			QueueConfig: QueueConfig{
				Name:       "",
				Durable:    false,
				AutoDelete: false,
			},
		},
		context:  context.TODO(),
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}

	err = channel.ExchangeDeclare(a.config.ExchangeConfig.Name, a.config.ExchangeConfig.Type, nil)

	if err != nil {
		t.Errorf("Failed to declare an exchange")
	}

	queue, err := channel.QueueDeclare("", wabbit.Option{
		"durable": a.config.QueueConfig.Durable,
		"delete":  a.config.QueueConfig.AutoDelete,
	})
	if err != nil {
		t.Errorf(err.Error())
	}

	if err := channel.Confirm(false); err != nil {
		t.Fatalf("[x] Channel could not be put into confirm mode: %s", err)
	}

	ctx, cancelFunc := context.WithDeadline(context.TODO(), time.Now().Add(100*time.Millisecond))
	defer cancelFunc()

	err = a.PollForMessages(&channel, &queue, ctx.Done())

	if err != nil {
		t.Errorf("testing err %s", err)
	}

	channel.Close()
	fakeServer.Stop()
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
		handler: sinkAccepted,
	}

	sinkServer := httptest.NewServer(h)
	defer sinkServer.Close()

	s, err := kncloudevents.NewHTTPMessageSenderWithTarget(sinkServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	statsReporter, _ := source.NewStatsReporter()
	a := NewAdapter(ctx, env, s, statsReporter)
	cmpA := &Adapter{
		config:            env.(*adapterConfig),
		httpMessageSender: s,
		reporter:          statsReporter,
		logger:            logging.FromContext(ctx).Desugar(),
		context:           ctx,
	}

	if a == cmpA {
		t.Errorf("Error in NewnvConfig return Type")
	}
}
