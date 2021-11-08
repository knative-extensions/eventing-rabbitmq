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

package rabbitmq

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/NeowayLabs/wabbit"
	"github.com/NeowayLabs/wabbit/amqp"
	"github.com/NeowayLabs/wabbit/amqptest"
	"github.com/NeowayLabs/wabbit/amqptest/server"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	origamqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/metrics/source"
	"knative.dev/pkg/logging"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink              func(http.ResponseWriter, *http.Request)
		reqBody           string
		attributes        map[string]string
		expectedEventType string
		withMsgId         bool
		error             bool
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
					Topic:   "topic",
					Brokers: "amqp://guest:guest@localhost:5672/",
					ExchangeConfig: ExchangeConfig{
						TypeOf:      "topic",
						Durable:     true,
						AutoDeleted: false,
						Internal:    false,
						NoWait:      false,
					},
					QueueConfig: QueueConfig{
						Name:             "",
						Durable:          false,
						DeleteWhenUnused: false,
						Exclusive:        true,
						NoWait:           false,
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

			if tc.reqBody != string(h.body) {
				t.Errorf("Expected request body '%q', but got '%q' %s", tc.reqBody, h.body, err)
			}
		})
	}
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
			Topic:   "",
			Brokers: "amqp://localhost:5672/%2f",
			ExchangeConfig: ExchangeConfig{
				TypeOf:      "direct",
				Durable:     true,
				AutoDeleted: false,
				Internal:    false,
				NoWait:      false,
			},
			QueueConfig: QueueConfig{
				Name:             "",
				Durable:          false,
				DeleteWhenUnused: false,
				Exclusive:        true,
				NoWait:           false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}

	conn, _ := a.CreateConn("", "", logging.FromContext(context.TODO()).Desugar())
	if conn != nil {
		t.Errorf("Failed to connect to RabbitMQ")
	}

	conn, _ = a.CreateConn("guest", "guest", logging.FromContext(context.TODO()).Desugar())
	if conn != nil {
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
			Topic:   "",
			Brokers: "amqp://localhost:5672/%2f",
			ExchangeConfig: ExchangeConfig{
				TypeOf:      "direct",
				Durable:     true,
				AutoDeleted: false,
				Internal:    false,
				NoWait:      false,
			},
			QueueConfig: QueueConfig{
				Name:             "",
				Durable:          false,
				DeleteWhenUnused: false,
				Exclusive:        true,
				NoWait:           false,
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

	/**
	Test for exchange type "direct"
	*/
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

	channel, err := conn.Channel()
	if err != nil {
		t.Errorf("Failed to open a channel")
	}

	a := &Adapter{
		config: &adapterConfig{
			Topic:   "",
			Brokers: "amqp://localhost:5672/%2f",
			ExchangeConfig: ExchangeConfig{
				TypeOf:      "direct",
				Durable:     true,
				AutoDeleted: false,
				Internal:    false,
				NoWait:      false,
			},
			QueueConfig: QueueConfig{
				Name:             "",
				Durable:          false,
				DeleteWhenUnused: false,
				Exclusive:        true,
				NoWait:           false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}

	_, err = a.StartAmqpClient(&channel)
	if err != nil {
		t.Errorf("Failed to start RabbitMQ")
	}

	/**
	Test for exchange type "fanout"
	*/
	a = &Adapter{
		config: &adapterConfig{
			Topic:   "",
			Brokers: "amqp://localhost:5672/%2f",
			ExchangeConfig: ExchangeConfig{
				TypeOf:      "fanout",
				Durable:     true,
				AutoDeleted: false,
				Internal:    false,
				NoWait:      false,
			},
			QueueConfig: QueueConfig{
				Name:             "",
				Durable:          false,
				DeleteWhenUnused: false,
				Exclusive:        true,
				NoWait:           false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}
	_, err = a.StartAmqpClient(&channel)
	if err != nil {
		t.Errorf("Failed to start RabbitMQ")
	}

	/**
	Test for exchange type "topic"
	*/
	a = &Adapter{
		config: &adapterConfig{
			Topic:   "",
			Brokers: "amqp://localhost:5672/%2f",
			ExchangeConfig: ExchangeConfig{
				TypeOf:      "topic",
				Durable:     true,
				AutoDeleted: false,
				Internal:    false,
				NoWait:      false,
			},
			QueueConfig: QueueConfig{
				Name:             "",
				Durable:          false,
				DeleteWhenUnused: false,
				Exclusive:        true,
				NoWait:           false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}
	_, err = a.StartAmqpClient(&channel)
	if err != nil {
		t.Errorf("Failed to start RabbitMQ")
	}

	fakeServer.Stop()
}

func TestAdapter_StartAmqpClient_InvalidExchangeType(t *testing.T) {
	/**
	Test for invalid exchange type
	*/
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
		t.Errorf("Failed to open a channel")
	}

	a := &Adapter{
		config: &adapterConfig{
			Topic:   "",
			Brokers: "amqp://localhost:5674/%2f",
			ExchangeConfig: ExchangeConfig{
				TypeOf:      "",
				Durable:     true,
				AutoDeleted: false,
				Internal:    false,
				NoWait:      false,
			},
			QueueConfig: QueueConfig{
				Name:             "",
				Durable:          false,
				DeleteWhenUnused: false,
				Exclusive:        true,
				NoWait:           false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}
	_, err = a.StartAmqpClient(&channel)
	if err != nil {
		t.Logf("Failed to start RabbitMQ")
	}
	fakeServer.Stop()
}

func TestAdapter_ConsumeMessages(t *testing.T) {
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
			Topic:   "",
			Brokers: "amqp://localhost:5672/%2f",
			ExchangeConfig: ExchangeConfig{
				TypeOf:      "direct",
				Durable:     true,
				AutoDeleted: false,
				Internal:    false,
				NoWait:      false,
			},
			QueueConfig: QueueConfig{
				Name:             "",
				Durable:          false,
				DeleteWhenUnused: false,
				Exclusive:        true,
				NoWait:           false,
			},
		},
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}
	queue, err := a.StartAmqpClient(&channel)
	if err != nil {
		t.Errorf("Failed to start RabbitMQ")
	}
	_, err = a.ConsumeMessages(&channel, queue, logging.FromContext(context.TODO()).Desugar())
	if err != nil {
		t.Errorf("Failed to consume from RabbitMQ")
	}
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
			Topic:   "topic",
			Brokers: "amqp://guest:guest@localhost:5672/",
			ExchangeConfig: ExchangeConfig{
				Name:        "Test-exchange",
				TypeOf:      "topic",
				Durable:     true,
				AutoDeleted: false,
				Internal:    false,
				NoWait:      false,
			},
			QueueConfig: QueueConfig{
				Name:             "",
				Durable:          false,
				DeleteWhenUnused: false,
				Exclusive:        true,
				NoWait:           false,
			},
		},
		context:  context.TODO(),
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}

	err = channel.ExchangeDeclare(a.config.ExchangeConfig.Name, a.config.ExchangeConfig.TypeOf, nil)

	if err != nil {
		t.Errorf("Failed to declare an exchange")
	}

	queue, err := channel.QueueDeclare("", wabbit.Option{
		"durable":   a.config.QueueConfig.Durable,
		"delete":    a.config.QueueConfig.DeleteWhenUnused,
		"exclusive": a.config.QueueConfig.Exclusive,
		"noWait":    a.config.QueueConfig.NoWait,
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

func TestAdapter_MsgIsBinary(t *testing.T) {
	for _, tt := range []struct {
		name       string
		msgHeaders wabbit.Option
		want       bool
	}{{
		name:       "msg with empty headers",
		msgHeaders: wabbit.Option{},
		want:       false,
	}, {
		name:       "msg is not binary",
		msgHeaders: wabbit.Option{"content-type": "test"},
		want:       false,
	}, {
		name:       "msg is binary",
		msgHeaders: wabbit.Option{"ce-specversion": "1.0"},
		want:       true,
	}, {
		name:       "msg is binary, titlecase",
		msgHeaders: wabbit.Option{"Ce-Specversion": "1.0"},
		want:       true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			got := isBinary(tt.msgHeaders)
			if got != tt.want {
				t.Errorf("Unexpected msg encoding for %s want:\n%v\ngot:\n%v", tt.msgHeaders, tt.want, got)
			}
		})
	}
}

func TestAdapter_MsgIsStructured(t *testing.T) {
	for _, tt := range []struct {
		name    string
		msgBody map[string]interface{}
		want    bool
	}{{
		name:    "msg with empty body",
		msgBody: map[string]interface{}{},
		want:    false,
	}, {
		name:    "msg is not structured",
		msgBody: map[string]interface{}{"data": "test"},
		want:    false,
	}, {
		name:    "msg is structured",
		msgBody: map[string]interface{}{"specversion": "1.0"},
		want:    true,
	}, {
		name:    "msg is structured, titlecase",
		msgBody: map[string]interface{}{"Specversion": "1.0"},
		want:    true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			got := isStructured(tt.msgBody)
			if got != tt.want {
				t.Errorf("Unexpected msg encoding for %s want:\n%v\ngot:\n%v", tt.msgBody, tt.want, got)
			}
		})
	}
}

func TestAdapter_MsgSetEventAttributes(t *testing.T) {
	// Specversion is mandatory in ce v1.0
	for _, tt := range []struct {
		name    string
		key     string
		ceKey   string
		val     interface{}
		want    string
		wantErr bool
	}{{
		name:  "empty string val",
		key:   "test",
		ceKey: "ce-test",
		val:   "",
		want:  `{"specversion": "1.0", "test": ""}`,
	}, {
		name:    "nil val err",
		key:     "test",
		ceKey:   "ce-test",
		val:     nil,
		want:    `{"specversion": "1.0"}`,
		wantErr: true,
	}, {
		name:    "wrong cloudevents key with '-'",
		key:     "wrong-key",
		ceKey:   "ce-wrong-key",
		val:     "test",
		want:    `{"specversion": "1.0"}`,
		wantErr: true,
	}, {
		name:  "set extension attribute",
		key:   "test",
		ceKey: "ce-test",
		val:   "test",
		want:  `{"specversion": "1.0", "test": "test"}`,
	}, {
		name:  "set cloudEvents attribute",
		key:   "source",
		ceKey: "ce-source",
		val:   "example/source.uri",
		want:  `{"specversion": "1.0", "source": "example/source.uri"}`,
	}, {
		name:  "floating numbers aproximation conversion to string",
		key:   "floatVal",
		ceKey: "ce-floatVal",
		val:   0.34,
		want:  `{"specversion": "1.0", "floatVal": 0}`,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			wantEvent := cloudevents.NewEvent()
			err := json.Unmarshal([]byte(tt.want), &wantEvent)
			if err != nil {
				t.Error(err)
			}

			gotEvent := cloudevents.NewEvent()
			err = setEventAttributes(&gotEvent, tt.key, tt.ceKey, tt.val)
			if err != nil && !tt.wantErr {
				t.Errorf("Unexpected error for:\nkey: %s, ceKey:%s, val: %s\nerr: %s", tt.key, tt.ceKey, tt.val, err)
			}

			if !tt.wantErr && gotEvent.String() != wantEvent.String() {
				t.Errorf("Unexpected event translation want:\n%v\ngot:\n%v", wantEvent, gotEvent)
			}
		})
	}
}

func TestAdapter_SetBinaryMessageProperties(t *testing.T) {
	for _, tt := range []struct {
		name       string
		msgHeaders wabbit.Option
		want       string
		wantErr    bool
	}{{
		name:       "msg with empty headers",
		msgHeaders: wabbit.Option{},
		want:       `{"specversion": "1.0"}`,
	}, {
		name:       "malformed cloudevents key",
		msgHeaders: wabbit.Option{"ce-wrong-key": "test"},
		want:       `{"specversion": "1.0"}`,
		wantErr:    true,
	}, {
		name:       "msg with one extension with floating value translated to lower key",
		msgHeaders: wabbit.Option{"Ce-Test": 0.3},
		want:       `{"specversion": "1.0", "test": "0"}`,
	}, {
		name:       "msg with one cloudevent attribute",
		msgHeaders: wabbit.Option{"ce-source": "example/source.uri"},
		want:       `{"specversion": "1.0", "source": "example/source.uri"}`,
	}, {
		name:       "msg ignore not ce- prefixed headers",
		msgHeaders: wabbit.Option{"test": "test"},
		want:       `{"specversion": "1.0"}`,
	}, {
		name:       "multi case event translation",
		msgHeaders: wabbit.Option{"test": "test", "ce-source": "example/source.uri", "ce-test": 0.6},
		want:       `{"specversion": "1.0", "source": "example/source.uri", "test": "0"}`,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			wantEvent := cloudevents.NewEvent()
			err := json.Unmarshal([]byte(tt.want), &wantEvent)
			if err != nil {
				t.Error(err)
			}

			gotEvent := cloudevents.NewEvent()
			err = setBinaryMessageProperties(&gotEvent, tt.msgHeaders)
			if err != nil && !tt.wantErr {
				t.Errorf("Unexpected error for:\nheaders: %s", tt.msgHeaders)
			}

			if gotEvent.String() != wantEvent.String() {
				t.Errorf("Unexpected binary event properties set:\n%v\ngot:\n%v", wantEvent, gotEvent)
			}
		})
	}
}

func TestAdapter_SetStructuredMessageProperties(t *testing.T) {
	for _, tt := range []struct {
		name    string
		msgBody map[string]interface{}
		want    string
	}{{
		name:    "msg with empty headers",
		msgBody: map[string]interface{}{},
		want:    `{"specversion": "1.0"}`,
	}, {
		name:    "msg with one extension with floating value",
		msgBody: map[string]interface{}{"test": 0.3},
		want:    `{"specversion": "1.0", "test": "0"}`,
	}, {
		name:    "msg transalate to lower key valid ce keys",
		msgBody: map[string]interface{}{"Source": "example/source.uri"},
		want:    `{"specversion": "1.0", "source": "example/source.uri"}`,
	}, {
		name:    "multi case event translation",
		msgBody: map[string]interface{}{"source": "example/source.uri", "test": 0.6},
		want:    `{"specversion": "1.0", "source": "example/source.uri", "test": "0"}`,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			wantEvent := cloudevents.NewEvent()
			err := json.Unmarshal([]byte(tt.want), &wantEvent)
			if err != nil {
				t.Error(err)
			}

			gotEvent := cloudevents.NewEvent()
			err = setStructuredMessageProperties(&gotEvent, tt.msgBody)
			if err != nil {
				t.Errorf("Unexpected error for:\nheaders: %s", tt.msgBody)
			}

			if gotEvent.String() != wantEvent.String() {
				t.Errorf("Unexpected binary event properties set:\n%v\ngot:\n%v", wantEvent, gotEvent)
			}
		})
	}
}
