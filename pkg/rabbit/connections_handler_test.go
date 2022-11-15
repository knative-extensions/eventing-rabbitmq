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

package rabbit

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func Test_SetupRabbitMQConnectionsError(t *testing.T) {
	rabbitMQHelper := NewRabbitMQHelper(2, make(chan bool), DialWrapper).(*RabbitMQHelper)
	logger := zap.NewNop().Sugar()
	testChannel := make(chan bool)
	// Drain messages in the retryChannel
	go Watcher(testChannel, *rabbitMQHelper)
	<-testChannel
	// Testing a failing setup
	conn, _, err := rabbitMQHelper.SetupRabbitMQ("amqp://localhost:5672/%2f", nil, logger)
	<-testChannel
	if err == nil {
		t.Error("SetupRabbitMQ should fail with the default DialFunc in testing environments")
	}
	if rabbitMQHelper.retryCounter == 0 {
		t.Errorf("no retries have been attempted want: > 0, got: %d", rabbitMQHelper.retryCounter)
	}
	rabbitMQHelper.CleanupRabbitMQ(conn, logger)
}

func Test_SetupRabbitMQChannelError(t *testing.T) {
	rabbitMQHelper := NewRabbitMQHelper(1, make(chan bool), ValidConnectionDial).(*RabbitMQHelper)
	logger := zap.NewNop().Sugar()
	testChannel := make(chan bool)
	go Watcher(testChannel, *rabbitMQHelper)
	<-testChannel
	conn, _, err := rabbitMQHelper.SetupRabbitMQ("amqp://localhost:5672/%2f", nil, logger)
	<-testChannel
	if err == nil {
		t.Error("SetupRabbitMQ should fail when creating a channel when using ValidConnectionDial")
	}
	rabbitMQHelper.CleanupRabbitMQ(conn, logger)
}

func Test_ValidSetupRabbitMQ(t *testing.T) {
	rabbitMQHelper := NewRabbitMQHelper(1, make(chan bool), ValidDial).(*RabbitMQHelper)
	logger := zap.NewNop().Sugar()
	testChannel := make(chan bool)
	go Watcher(testChannel, *rabbitMQHelper)
	<-testChannel
	conn, channel, err := rabbitMQHelper.SetupRabbitMQ("amqp://localhost:5672/%2f", ConfigTest, logger)
	if err != nil {
		t.Errorf("Setup should'nt fail when using ValidDial func %s", err)
	}
	channel.(*RabbitMQChannelMock).NotifyCloseChannel <- amqp.ErrClosed
	conn.Close()
}

func ConfigTest(conn RabbitMQConnectionInterface, channel RabbitMQChannelInterface) error {
	ChannelConfirm(conn, channel)
	ChannelQoS(conn, channel)
	return nil
}

func TestAdapter_VhostHandler(t *testing.T) {
	for _, tt := range []struct {
		name   string
		broker string
		vhost  string
		want   string
	}{{
		name:   "no broker nor vhost",
		broker: "",
		vhost:  "",
		want:   "",
	}, {
		name:   "no vhost",
		broker: "amqp://localhost:5672",
		vhost:  "",
		want:   "amqp://localhost:5672",
	}, {
		name:   "no broker",
		broker: "",
		vhost:  "test-vhost",
		want:   "test-vhost",
	}, {
		name:   "broker and vhost without separating slash",
		broker: "amqp://localhost:5672",
		vhost:  "test-vhost",
		want:   "amqp://localhost:5672/test-vhost",
	}, {
		name:   "broker and vhost without separating slash but vhost with ending slash",
		broker: "amqp://localhost:5672",
		vhost:  "test-vhost/",
		want:   "amqp://localhost:5672/test-vhost/",
	}, {
		name:   "broker with trailing slash and vhost without the slash",
		broker: "amqp://localhost:5672/",
		vhost:  "test-vhost",
		want:   "amqp://localhost:5672/test-vhost",
	}, {
		name:   "vhost starting with slash and broker without the slash",
		broker: "amqp://localhost:5672",
		vhost:  "/test-vhost",
		want:   "amqp://localhost:5672/test-vhost",
	}, {
		name:   "broker and vhost with slash",
		broker: "amqp://localhost:5672/",
		vhost:  "/test-vhost",
		want:   "amqp://localhost:5672//test-vhost",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			got := VHostHandler(tt.broker, tt.vhost)
			if got != tt.want {
				t.Errorf("Unexpected URI for %s/%s want:\n%+s\ngot:\n%+s", tt.broker, tt.vhost, tt.want, got)
			}
		})
	}
}
