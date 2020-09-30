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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/NeowayLabs/wabbit"
	"github.com/NeowayLabs/wabbit/amqptest/server"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

const (
	rabbitURL    = "amqp://localhost:5672/%2f"
	queueName    = "queue"
	exchangeName = "default/knative-testbroker"
	expectedBody = `"{\"testdata\":\"testdata\"}"`
)

type fakeHandler struct {
	done   chan bool
	mu     sync.Mutex
	body   string
	header http.Header

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) setBody(body string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.body = body
}

func (h *fakeHandler) getBody() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.body
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.header = r.Header
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.setBody(string(body))

	defer r.Body.Close()
	h.handler(w, r)
	h.done <- true
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func TestEndToEnd(t *testing.T) {
	backoffDelay, err := time.ParseDuration("1s")
	if err != nil {
		t.Error("Failed to parse duration: ", err)
	}

	subscriberDone := make(chan bool, 1)
	subscriberHandler := &fakeHandler{
		handler: sinkAccepted,
		done:    subscriberDone,
	}
	subscriber := httptest.NewServer(subscriberHandler)
	defer subscriber.Close()

	brokerHandler := &fakeHandler{
		handler: sinkAccepted,
	}
	broker := httptest.NewServer(brokerHandler)
	defer broker.Close()

	fakeServer := server.NewServer(rabbitURL)
	err = fakeServer.Start()
	if err != nil {
		t.Errorf("%s: %s", "Failed to start RabbitMQ", err)
	}
	vh := server.NewVHost("/")

	ch := server.NewChannel(vh)
	err = ch.ExchangeDeclare(exchangeName, "headers", // kind
		wabbit.Option{
			"durable":    false,
			"autoDelete": false,
			"internal":   false,
			"noWait":     false,
		},
	)
	if err != nil {
		t.Error(err)
		return
	}

	_, err = ch.QueueDeclare(queueName,
		wabbit.Option{
			"durable":    false,
			"autoDelete": false,
			"exclusive":  false,
			"noWait":     false,
		},
	)

	if err != nil {
		t.Error(err)
		return
	}

	err = ch.QueueBind(queueName, "process.data", exchangeName, nil)

	if err != nil {
		t.Error(err)
		return
	}
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID("test-id")
	event.SetType("testtype")
	event.SetSource("testsource")
	event.SetSubject(fmt.Sprintf("%s-%s", event.Source(), event.Type()))
	event.SetData(cloudevents.ApplicationJSON, `{"testdata":"testdata"}`)
	bytes, err := json.Marshal(event)
	if err != nil {
		t.Error("Failed to marshal the event: ", err)
	}

	err = ch.Publish(exchangeName, "process.data", bytes, nil)

	if err != nil {
		t.Error(err)
		return
	}

	d := NewDispatcher(broker.URL, subscriber.URL, false /*requeue*/, 0, backoffDelay, eventingduckv1.BackoffPolicyLinear)
	go func() {
		d.ConsumeFromQueue(context.TODO(), ch, queueName)
	}()

	<-subscriberDone
	if diff := cmp.Diff(expectedBody, subscriberHandler.getBody()); diff != "" {
		t.Errorf("unexpected diff (-want, +got) = ", diff)
	}
}
