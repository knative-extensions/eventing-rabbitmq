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
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type RabbitMQConnectionMock struct{}

func (rm *RabbitMQConnectionMock) ChannelWrapper() (RabbitMQChannelInterface, error) {
	return &RabbitMQChannelMock{}, nil
}

func (rm *RabbitMQConnectionMock) IsClosed() bool {
	return true
}

func (rm *RabbitMQConnectionMock) Close() error {
	return nil
}

func (rm *RabbitMQConnectionMock) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	return c
}

type RabbitMQBadConnectionMock struct{}

func (rm *RabbitMQBadConnectionMock) ChannelWrapper() (RabbitMQChannelInterface, error) {
	return nil, errors.New("channel error test")
}

func (rm *RabbitMQBadConnectionMock) IsClosed() bool {
	return false
}

func (rm *RabbitMQBadConnectionMock) Close() error {
	return nil
}

func (rm *RabbitMQBadConnectionMock) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	return c
}

type RabbitMQChannelMock struct {
	NotifyCloseChannel chan *amqp.Error
}

func (rm *RabbitMQChannelMock) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	rm.NotifyCloseChannel = c
	return c
}

func (rm *RabbitMQChannelMock) Qos(a int, b int, c bool) error {
	return nil
}

func (rm *RabbitMQChannelMock) Consume(a string, b string, c bool, d bool, e bool, f bool, t amqp.Table) (<-chan amqp.Delivery, error) {
	return make(<-chan amqp.Delivery), nil
}

func (rm *RabbitMQChannelMock) PublishWithDeferredConfirm(a string, b string, c bool, d bool, p amqp.Publishing) (*amqp.DeferredConfirmation, error) {
	return &amqp.DeferredConfirmation{}, nil
}

func (rm *RabbitMQChannelMock) Confirm(a bool) error {
	return nil
}

func (rm *RabbitMQChannelMock) QueueInspect(string) (amqp.Queue, error) {
	return amqp.Queue{}, nil
}

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

	for channel.(*RabbitMQChannelMock).NotifyCloseChannel == nil {
	}
	channel.(*RabbitMQChannelMock).NotifyCloseChannel <- amqp.ErrClosed
	conn.Close()
}

func ValidConnectionDial(url string) (RabbitMQConnectionInterface, error) {
	return &RabbitMQConnection{connection: &RabbitMQBadConnectionMock{}}, nil
}

func ValidDial(url string) (RabbitMQConnectionInterface, error) {
	return &RabbitMQConnection{connection: &RabbitMQConnectionMock{}}, nil
}

func Watcher(testChannel chan bool, rabbitmqHelper RabbitMQHelper) {
	testChannel <- true
	for {
		retry := rabbitmqHelper.WaitForRetrySignal()
		if !retry {
			close(testChannel)
			break
		}
		testChannel <- retry
	}
}

func ConfigTest(conn RabbitMQConnectionInterface, channel RabbitMQChannelInterface) error {
	ChannelConfirm(conn, channel)
	ChannelQoS(conn, channel)
	return nil
}
