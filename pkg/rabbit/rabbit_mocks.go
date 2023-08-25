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

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQConnectionMock struct{ NotifyCloseChannel chan *amqp.Error }

func (rm *RabbitMQConnectionMock) ChannelWrapper() (RabbitMQChannelInterface, error) {
	return &RabbitMQChannelMock{NotifyCloseChannel: make(chan *amqp.Error)}, nil
}

func (rm *RabbitMQConnectionMock) IsClosed() bool {
	return false
}

func (rm *RabbitMQConnectionMock) Close() error {
	return nil
}

func (rm *RabbitMQConnectionMock) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	return rm.NotifyCloseChannel
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

type RabbitMQChannelMock struct {
	NotifyCloseChannel chan *amqp.Error
	ConsumeChannel     <-chan amqp.Delivery
}

func (rm *RabbitMQChannelMock) IsClosed() bool {
	return false
}

func (rm *RabbitMQChannelMock) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	return rm.NotifyCloseChannel
}

func (rm *RabbitMQChannelMock) Qos(a int, b int, c bool) error {
	return nil
}

func (rm *RabbitMQChannelMock) Consume(a string, b string, c bool, d bool, e bool, f bool, t amqp.Table) (<-chan amqp.Delivery, error) {
	if rm.ConsumeChannel == nil {
		rm.ConsumeChannel = make(<-chan amqp.Delivery)
	}
	return rm.ConsumeChannel, nil
}

func (rm *RabbitMQChannelMock) PublishWithDeferredConfirm(a string, b string, c bool, d bool, p amqp.Publishing) (*amqp.DeferredConfirmation, error) {
	return &amqp.DeferredConfirmation{}, nil
}

func (rm *RabbitMQChannelMock) Confirm(a bool) error {
	return nil
}

func (rm *RabbitMQChannelMock) QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	return amqp.Queue{}, nil
}

func BadConnectionDial(url string) (RabbitMQConnectionWrapperInterface, error) {
	return nil, errors.New("connection error test")
}

func BadChannelDial(url string) (RabbitMQConnectionWrapperInterface, error) {
	return NewConnection(&RabbitMQBadConnectionMock{}), nil
}

func ValidDial(url string) (RabbitMQConnectionWrapperInterface, error) {
	return NewConnection(&RabbitMQConnectionMock{NotifyCloseChannel: make(chan *amqp.Error)}), nil
}
