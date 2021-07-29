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
	origamqp "github.com/streadway/amqp"
	"go.uber.org/zap"
	sourcesv1alpha1 "knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
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
		error             bool
	}{
		"accepted": {
			sink:    sinkAccepted,
			reqBody: `{"key":"value"}`,
		},
		"rejected": {
			sink:    sinkRejected,
			reqBody: `{"key":"value"}`,
			error:   true,
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

			data, err := json.Marshal(map[string]string{"key": "value"})
			if err != nil {
				t.Errorf("unexpected error, %v", err)
			}

			m := &amqp.Delivery{}
			m.Delivery = &origamqp.Delivery{
				MessageId: "id",
				Body:      data,
				Headers:   origamqp.Table{"content-type": "application/json"},
			}
			err = a.postMessage(m)

			if tc.error && err == nil {
				t.Errorf("expected error, but got %v", err)
			}

			if tc.reqBody != string(h.body) {
				t.Errorf("Expected request body '%q', but got '%q'", tc.reqBody, h.body)
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

func TestAdapterVhostHandler(t *testing.T) {
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

func TestAdapter_msgContentType(t *testing.T) {
	for _, tt := range []struct {
		name        string
		contentType string
		want        string
		header      string
		err         bool
	}{{
		name:        "cloudevents json message content type",
		contentType: "application/cloudevents+json",
		want:        "application/cloudevents+json",
	}, {
		name:        "clouidevents batch message content type",
		contentType: "application/cloudevents-batch+json",
		want:        "application/cloudevents-batch+json",
	}, {
		name:        "text plain message content type",
		contentType: "text/plain",
		want:        "text/plain",
	}, {
		name:        "case insensitive message conte type",
		contentType: "Application/XML",
		want:        "application/xml",
	}, {
		name:        "case insensitive content type header",
		contentType: "text/json",
		want:        "text/json",
		header:      "Content-Type",
	}, {
		name:        "empty content type error",
		contentType: "",
		want:        "",
		err:         true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			header := "content-type"
			if tt.header != "" || tt.err {
				header = tt.header
			}

			m := &amqp.Delivery{}
			m.Delivery = &origamqp.Delivery{
				MessageId: "id",
				Body:      nil,
				Headers:   origamqp.Table{header: tt.contentType},
			}

			got, err := msgContentType(m)
			if err != nil && got != tt.want {
				t.Errorf("Unexpected message content type want:\n%+s\ngot:\n%+s", tt.want, got)
			}

			if tt.err && err == nil {
				t.Errorf("Unexpected error state for msg type want:\n%+s\ngot:\n%+s", tt.want, got)
			}
		})
	}
}

func TestAdapter_setEventContent(t *testing.T) {
	statsReporter, _ := source.NewStatsReporter()

	a := &Adapter{
		config: &adapterConfig{
			Topic:   "topic",
			Brokers: "amqp://guest:guest@localhost:5672/",
		},
		context:  context.TODO(),
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}

	for _, tt := range []struct {
		name        string
		contentType string
		msgId       string
	}{{
		name:        "maintain msg id on message processing",
		contentType: "application/json",
		msgId:       "1234",
	}, {
		name:        "set new id is message id is empty",
		contentType: "application/json",
		msgId:       "",
	}, {
		name:        "sent event as it is when its type is cloudevent",
		contentType: "application/cloudevents+json",
		msgId:       "1234",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			var data []byte
			var err error
			var event cloudevents.Event

			msgId := ""
			if tt.msgId != "" {
				msgId = tt.msgId
			}

			if tt.contentType == "application/cloudevents+json" {
				event = cloudevents.NewEvent()
				event.SetID(tt.msgId)
				event.SetType(sourcesv1alpha1.RabbitmqEventType)
				event.SetSource(sourcesv1alpha1.RabbitmqEventSource(
					a.config.Namespace,
					a.config.Name,
					a.config.Topic,
				))
				event.SetSubject(tt.msgId)
				event.SetExtension("key", tt.msgId)
				data, err = json.Marshal(event)
			} else {
				data, err = json.Marshal(map[string]string{"key": "value"})
			}

			if err != nil {
				t.Errorf("unexpected error, %v", err)
			}

			m := &amqp.Delivery{}
			m.Delivery = &origamqp.Delivery{
				MessageId: msgId,
				Body:      data,
				Headers:   origamqp.Table{"content-type": tt.contentType},
			}

			got, _ := setEventContent(a, m, tt.contentType)
			if tt.msgId != "" && got.Context.GetID() != tt.msgId {
				t.Errorf("Unexpected message ID want:\n%+s\ngot:\n%+s", tt.msgId, got.Context.GetID())
			}

			if tt.msgId == "" && got.Context.GetID() == tt.msgId {
				t.Errorf("Unexpected message ID want:\nany ID\ngot:\n%+s", got.Context.GetID())
			}

			if tt.contentType == "application/cloudevents+json" && got.String() != event.String() {
				t.Errorf(
					"Error passing cloudevent as it is want:\n%+s ID\ngot:\n%+s",
					event.String(), got.String(),
				)
			}
		})
	}
}

func TestAdapter_setEventBatchContent(t *testing.T) {
	statsReporter, _ := source.NewStatsReporter()

	a := &Adapter{
		config: &adapterConfig{
			Topic:   "topic",
			Brokers: "amqp://guest:guest@localhost:5672/",
		},
		context:  context.TODO(),
		logger:   zap.NewNop(),
		reporter: statsReporter,
	}
	msgId := "1234"
	event := cloudevents.NewEvent()
	event.SetID(msgId)
	event.SetType(sourcesv1alpha1.RabbitmqEventType)
	event.SetSource(sourcesv1alpha1.RabbitmqEventSource(
		a.config.Namespace,
		a.config.Name,
		a.config.Topic,
	))
	event.SetSubject(msgId)
	event.SetExtension("key", msgId)
	payload := []cloudevents.Event{event, event.Clone(), event.Clone()}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Errorf("unexpected error, %v", err)
	}

	m := &amqp.Delivery{}
	m.Delivery = &origamqp.Delivery{
		MessageId: msgId,
		Body:      data,
		Headers:   origamqp.Table{"content-type": "application/cloudevents-batch+json"},
	}

	got, _ := setEventBatchContent(a, m)
	for i, event := range got {
		if event.String() != payload[i].String() {
			t.Errorf("Error converting event batch want:\n%+s ID\ngot:\n%+s", event.String(), payload[i].String())
		}
	}
}
