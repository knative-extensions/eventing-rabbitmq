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
	"bytes"
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/NeowayLabs/wabbit"
	"github.com/NeowayLabs/wabbit/amqptest/server"
)

const (
	rabbitURL    = "amqp://localhost:5672/%2f"
	queueName    = "queue"
	exchangeName = "default/knative-testbroker"
	/*eventData            = `{"testdata":"testdata"}`
	eventData2           = `{"testdata":"testdata2"}`
	responseData         = `{"testresponse":"testresponsedata"}`
	expectedData         = `"{\"testdata\":\"testdata\"}"`
	expectedData2        = `"{\"testdata\":\"testdata2\"}"`
	expectedResponseData = `"{\"testresponse\":\"testresponsedata\"}"`*/
)

func TestFailToConsume(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)

	ch, _, err := createRabbitAndQueue()
	if err != nil {
		t.Error("Failed to create rabbit and queue")
	}
	d := &Dispatcher{}
	err = d.ConsumeFromQueue(context.TODO(), ch, "nosuchqueue")
	if err == nil {
		t.Fatal("Did not fail to consume.", err)
	}

	if err.Error() != "create consumer: Unknown queue 'nosuchqueue'" {
		t.Fatal("Unexpected failure message, got: ", err)
	}
}

func createRabbitAndQueue() (wabbit.Channel, *server.AMQPServer, error) {
	fakeServer := server.NewServer(rabbitURL)
	err := fakeServer.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start RabbitMQ: %s", err)
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
		fakeServer.Stop()
		return nil, nil, fmt.Errorf("failed to declare exchange: %s", err)
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
		ch.Close()
		fakeServer.Stop()
		return nil, nil, fmt.Errorf("failed to declare Queue: %s", err)
	}

	err = ch.QueueBind(queueName, "process.data", exchangeName, nil)

	if err != nil {
		ch.Close()
		fakeServer.Stop()
		return nil, nil, fmt.Errorf("failed to bind Queue: %s", err)
	}
	return ch, fakeServer, nil
}

/* func TestEndToEnd(t *testing.T) {
	t.Skip()
	testCases := map[string]struct {
		// Subscriber config, how many events to expect, how to respond, etc.
		subscriberReceiveCount int
		subscriberHandlers     []handlerFunc
		responseEvents         []ce.Event

		// Broker config, how many events to expect, how to respond, etc.
		brokerReceiveCount int
		brokerHandlers     []handlerFunc

		// Delivery configuration
		maxRetries    int
		backoffPolicy eventingduckv1.BackoffPolicyType
		// timeout for dispatcher
		timeout time.Duration

		// Processing time for subscribers
		processingTime []time.Duration

		// Cloud Events to queue to Rabbit
		events []ce.Event
		// Raw bytes to queue to Rabbit. These get queued before cloud events.
		rawMessages [][]byte

		// Messages we expect Subscriber / Broker to receive
		expectedSubscriberBodies []string
		expectedBrokerBodies     []string

		// 	any error we expect from consuming from the queue
		consumeErr error
	}{
		// ** No requeues **
		"One event, success, no response": {
			subscriberReceiveCount:   1,
			subscriberHandlers:       []handlerFunc{accepted},
			events:                   []ce.Event{createEvent(eventData)},
			expectedSubscriberBodies: []string{expectedData},
			consumeErr:               context.Canceled,
		},
		"One event, failure, no response": {
			subscriberReceiveCount:   1,
			subscriberHandlers:       []handlerFunc{failed},
			events:                   []ce.Event{createEvent(eventData)},
			expectedSubscriberBodies: []string{expectedData},
			consumeErr:               context.Canceled,
		},
		"two events, success, no response": {
			subscriberReceiveCount:   2,
			subscriberHandlers:       []handlerFunc{accepted, accepted},
			events:                   []ce.Event{createEvent(eventData), createEvent(eventData2)},
			expectedSubscriberBodies: []string{expectedData, expectedData2},
			consumeErr:               context.Canceled,
		},
		"two events, first malformed, one delivered": {
			subscriberReceiveCount:   1,
			subscriberHandlers:       []handlerFunc{accepted},
			rawMessages:              [][]byte{[]byte("garbage")},
			events:                   []ce.Event{createEvent(eventData2)},
			expectedSubscriberBodies: []string{expectedData2},
			consumeErr:               context.Canceled,
		},
		"One event, success, response, goes to broker, accepted": {
			subscriberReceiveCount:   1,
			subscriberHandlers:       []handlerFunc{accepted},
			events:                   []ce.Event{createEvent(eventData)},
			expectedSubscriberBodies: []string{expectedData},
			responseEvents:           []ce.Event{createEvent(responseData)},
			brokerReceiveCount:       1,
			brokerHandlers:           []handlerFunc{accepted},
			expectedBrokerBodies:     []string{expectedResponseData},
			consumeErr:               context.Canceled,
		},
		"One event, success, response, goes to broker, failed": {
			subscriberReceiveCount:   1,
			subscriberHandlers:       []handlerFunc{accepted},
			events:                   []ce.Event{createEvent(eventData)},
			expectedSubscriberBodies: []string{expectedData},
			responseEvents:           []ce.Event{createEvent(responseData)},
			brokerReceiveCount:       1,
			brokerHandlers:           []handlerFunc{failed},
			expectedBrokerBodies:     []string{expectedResponseData},
			consumeErr:               context.Canceled,
		},
		"One event, long processing time, failed": {
			subscriberReceiveCount:   1,
			subscriberHandlers:       []handlerFunc{accepted},
			timeout:                  time.Second * 2,
			processingTime:           []time.Duration{time.Second * 5},
			responseEvents:           []ce.Event{createEvent(responseData)},
			events:                   []ce.Event{createEvent(eventData)},
			expectedSubscriberBodies: []string{expectedData},
			consumeErr:               context.Canceled,
		},
		// ** With retries **
		"One event, 2 failures, 3rd one succeeds no response, linear retry, 2 retries": {
			subscriberReceiveCount:   3,
			subscriberHandlers:       []handlerFunc{failed, failed, accepted},
			events:                   []ce.Event{createEvent(eventData)},
			expectedSubscriberBodies: []string{expectedData, expectedData, expectedData},
			maxRetries:               2,
			consumeErr:               context.Canceled,
			backoffPolicy:            eventingduckv1.BackoffPolicyLinear,
		},
		"One event, success, response, goes to broker, failed once, retried once, then accepted": {
			subscriberReceiveCount:   1,
			subscriberHandlers:       []handlerFunc{accepted},
			events:                   []ce.Event{createEvent(eventData)},
			expectedSubscriberBodies: []string{expectedData},
			responseEvents:           []ce.Event{createEvent(responseData)},
			brokerReceiveCount:       2,
			brokerHandlers:           []handlerFunc{failed, accepted},
			expectedBrokerBodies:     []string{expectedResponseData, expectedResponseData},
			maxRetries:               1,
			backoffPolicy:            eventingduckv1.BackoffPolicyLinear,
			consumeErr:               context.Canceled,
		},
	}

	backoffDelay, err := time.ParseDuration("1s")
	if err != nil {
		t.Error("Failed to parse duration: ", err)
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			subscriberDone := make(chan bool, 1)
			subscriberHandler := &fakeHandler{
				handlers:       tc.subscriberHandlers,
				done:           subscriberDone,
				exitAfter:      tc.subscriberReceiveCount,
				responseEvents: tc.responseEvents,
				processingTime: tc.processingTime,
			}
			subscriber := httptest.NewServer(subscriberHandler)
			defer subscriber.Close()

			brokerDone := make(chan bool, 1)
			brokerHandler := &fakeHandler{
				handlers:  tc.brokerHandlers,
				done:      brokerDone,
				exitAfter: tc.brokerReceiveCount,
			}
			broker := httptest.NewServer(brokerHandler)
			defer broker.Close()

			ch, srv, err := createRabbitAndQueue()
			if err != nil {
				t.Error("Failed to create Rabbit and queue:", err)
			}

			for i := range tc.rawMessages {
				err = ch.Publish(exchangeName, "process.data", tc.rawMessages[i], wabbit.Option{})
				if err != nil {
					t.Errorf("Failed to publish raw message %d: %s", i, err)
				}
			}
			for i, event := range tc.events {
				headers := wabbit.Option{
					"content-type":   event.DataContentType(),
					"ce-specversion": event.SpecVersion(),
					"ce-type":        event.Type(),
					"ce-source":      event.Source(),
					"ce-subject":     event.Subject(),
					"ce-id":          event.ID(),
				}

				for key, val := range event.Extensions() {
					headers[key] = val
				}

				err = ch.Publish(exchangeName, "process.data", event.Data(), wabbit.Option{
					"headers":     headers,
					"messageId":   event.ID(),
					"contentType": event.DataContentType()})
				if err != nil {
					t.Errorf("Failed to publish event %d: %s", i, err)
				}
			}

			var backoffPolicy eventingduckv1.BackoffPolicyType
			if tc.backoffPolicy == "exponential" || tc.backoffPolicy == "" {
				backoffPolicy = eventingduckv1.BackoffPolicyExponential
			} else {
				backoffPolicy = eventingduckv1.BackoffPolicyLinear

			}
			d := &Dispatcher{
				BrokerIngressURL: broker.URL,
				SubscriberURL:    subscriber.URL,
				MaxRetries:       tc.maxRetries,
				BackoffDelay:     backoffDelay,
				BackoffPolicy:    backoffPolicy,
				WorkerCount:      1,
				Timeout:          tc.timeout,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := d.ConsumeFromQueue(ctx, ch, queueName); err != tc.consumeErr {
					t.Errorf("unexpected consumer error, want %v got %v", tc.consumeErr, err)
				}

			}()

			brokerFinished := false
			// If Broker is not supposed to receive events, short circuit it.
			if tc.brokerReceiveCount == 0 {
				brokerDone <- true
			}
			subscriberFinished := false
			for !brokerFinished || !subscriberFinished {
				select {
				case <-subscriberDone:
					subscriberFinished = true
					t.Logf("Subscriber Done")
				case <-brokerDone:
					brokerFinished = true
					t.Logf("Broker Done")
				case <-time.After(40 * time.Second):
					t.Fatalf("Timed out the test. Subscriber or Broker did not get the wanted events: SubscriberFinished: %v BrokerFinished: %v", subscriberFinished, brokerFinished)
				}
			}

			// stop the consumer and wait
			cancel()
			wg.Wait()

			ch.Close()
			srv.Stop()
			if subscriberHandler.getReceivedCount() != tc.subscriberReceiveCount {
				t.Errorf("subscriber got %d events, wanted %d", subscriberHandler.getReceivedCount(), tc.subscriberReceiveCount)
			} else {
				if diff := cmp.Diff(tc.expectedSubscriberBodies, subscriberHandler.getBodies(), cmpopts.SortSlices(stringSort)); diff != "" {
					t.Error("unexpected subscriber diff (-want, +got) = ", diff)
				}
			}
			if brokerHandler.getReceivedCount() != tc.brokerReceiveCount {
				t.Errorf("broker got %d events, wanted %d", brokerHandler.getReceivedCount(), tc.brokerReceiveCount)
			} else {
				for i := range tc.expectedBrokerBodies {
					if diff := cmp.Diff(tc.expectedBrokerBodies[i], brokerHandler.getBodies()[i]); diff != "" {
						t.Error("unexpected broker diff (-want, +got) = ", diff)
					}
				}
			}
		})
	}
}

type fakeHandler struct {
	done   chan bool
	mu     sync.Mutex
	bodies []string
	header http.Header
	// How many events to receive before exiting.
	exitAfter    int
	receiveCount int

	// How long to wait before responding.
	processingTime []time.Duration

	// handlers for various requests
	handlers []handlerFunc

	// response events if any
	responseEvents []ce.Event
}


func (h *fakeHandler) getBodies() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.bodies
}

func (h *fakeHandler) getReceivedCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.receiveCount
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
	h.addBody(string(body))

	defer r.Body.Close()
	if len(h.responseEvents) > 0 {
		// write the response event out if there are any
		if len(h.processingTime) > 0 {
			time.Sleep(h.processingTime[h.receiveCount])
		}

		ev := h.responseEvents[h.receiveCount]
		w.Header()["ce-specversion"] = []string{"1.0"}
		w.Header()["ce-id"] = []string{ev.ID()}
		w.Header()["ce-type"] = []string{ev.Type()}
		w.Header()["ce-source"] = []string{ev.Source()}
		w.Header()["ce-subject"] = []string{ev.Subject()}
		w.Header()["content-type"] = []string{"application/json"}
		w.Write(ev.Data())
	} else {
		if len(h.processingTime) > 0 {
			h.handlers[h.receiveCount](w, r, h.processingTime[h.receiveCount])
		} else {
			h.handlers[h.receiveCount](w, r, 0)
		}
	}
	h.receiveCount++
	h.exitAfter--
	if h.exitAfter == 0 {
		h.done <- true
	}
}

type handlerFunc func(http.ResponseWriter, *http.Request, time.Duration)

func (h *fakeHandler) addBody(body string) {
	h.bodies = append(h.bodies, body)
}

func accepted(writer http.ResponseWriter, req *http.Request, delay time.Duration) {
	time.Sleep(delay)
	writer.WriteHeader(http.StatusOK)
}

func failed(writer http.ResponseWriter, req *http.Request, delay time.Duration) {
	time.Sleep(delay)
	writer.WriteHeader(500)
}

func createEvent(data string) ce.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID("test-id")
	event.SetType("testtype")
	event.SetSource("testsource")
	event.SetSubject(fmt.Sprintf("%s-%s", event.Source(), event.Type()))
	event.SetData(cloudevents.ApplicationJSON, data)
	return event
}

func stringSort(x, y string) bool {
	return x < y
} */
