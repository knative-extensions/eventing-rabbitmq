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
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type RabbitMQHelperInterface interface {
	SetupRabbitMQ(string, func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error, *zap.SugaredLogger) (RabbitMQConnectionInterface, RabbitMQChannelInterface, error)
	WatchRabbitMQConnections(RabbitMQConnectionInterface, RabbitMQChannelInterface, string, *zap.SugaredLogger)
	SignalRetry(bool)
	WaitForRetrySignal() bool
	CloseRabbitMQConnections(RabbitMQConnectionInterface, *zap.SugaredLogger)
	CleanupRabbitMQ(connection RabbitMQConnectionInterface, logger *zap.SugaredLogger)
}

type RabbitMQConnectionInterface interface {
	Channel() (*amqp.Channel, error)
	NotifyClose(chan *amqp.Error) chan *amqp.Error
	Close() error
	IsClosed() bool
}

type RabbitMQChannelInterface interface {
	NotifyClose(chan *amqp.Error) chan *amqp.Error
	Qos(int, int, bool) error
	Confirm(bool) error
	Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error)
	PublishWithDeferredConfirm(string, string, bool, bool, amqp.Publishing) (*amqp.DeferredConfirmation, error)
	QueueInspect(string) (amqp.Queue, error)
}

type RabbitMQHelper struct {
	retryCounter  int
	cycleDuration time.Duration
	cleaningUp    bool
	retryChannel  chan bool
	dialFunc      func(string) (RabbitMQConnectionInterface, error)
}

func NewRabbitMQHelper(cycleDuration time.Duration, retryChannel chan bool, dialFunc func(string) (RabbitMQConnectionInterface, error)) RabbitMQHelperInterface {
	return &RabbitMQHelper{
		cycleDuration: cycleDuration,
		retryChannel:  retryChannel,
		dialFunc:      dialFunc,
	}
}

func (r *RabbitMQHelper) SetupRabbitMQ(
	RabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error,
	logger *zap.SugaredLogger) (RabbitMQConnectionInterface, RabbitMQChannelInterface, error) {
	r.retryCounter += 1
	var err error
	var connInterface RabbitMQConnectionInterface
	var channelInterface RabbitMQChannelInterface
	if connInterface, err = r.dialFunc(RabbitMQURL); err != nil {
		logger.Errorw("failed to connect to RabbitMQ", zap.Error(err))
	} else if channelInterface, err = connInterface.Channel(); err != nil {
		logger.Errorw("failed to open a RabbitMQ channel", zap.Error(err))
	}

	if configFunction != nil && err == nil {
		err = configFunction(connInterface, channelInterface)
	}

	// If there is an error trying to setup rabbit send a retry msg
	if err != nil {
		logger.Warnf("retry number %d", r.retryCounter)
		time.Sleep(time.Second * r.cycleDuration)
		go r.SignalRetry(true)
		return nil, nil, err
	}

	// if there is no error reset the retryCounter and cycle values
	r.retryCounter = 0
	// Wait for a channel or connection close message to rerun the RabbitMQ setup
	go r.WatchRabbitMQConnections(connInterface, channelInterface, RabbitMQURL, logger)
	return connInterface, channelInterface, nil
}

func (r *RabbitMQHelper) WatchRabbitMQConnections(
	connection RabbitMQConnectionInterface,
	channel RabbitMQChannelInterface,
	RabbitMQURL string,
	logger *zap.SugaredLogger) {
	var err error
	select {
	case err = <-connection.NotifyClose(make(chan *amqp.Error)):
	case err = <-channel.NotifyClose(make(chan *amqp.Error)):
	}
	if !r.cleaningUp {
		logger.Warn("Lost connection to RabbitMQ, reconnecting. Error: %v", zap.Error(err))
		r.CloseRabbitMQConnections(connection, logger)
		r.SignalRetry(true)
	}
}

func (r *RabbitMQHelper) SignalRetry(retry bool) {
	r.retryChannel <- retry
}

func (r *RabbitMQHelper) WaitForRetrySignal() bool {
	return <-r.retryChannel
}

func (r *RabbitMQHelper) CloseRabbitMQConnections(connection RabbitMQConnectionInterface, logger *zap.SugaredLogger) {
	r.cleaningUp = true
	if connection != nil && !connection.IsClosed() {
		if err := connection.Close(); err != nil {
			logger.Error(err)
		}
	}
	r.cleaningUp = false
}

func (r *RabbitMQHelper) CleanupRabbitMQ(connection RabbitMQConnectionInterface, logger *zap.SugaredLogger) {
	r.SignalRetry(false)
	r.CloseRabbitMQConnections(connection, logger)
	close(r.retryChannel)
}

func ChannelQoS(connection RabbitMQConnectionInterface, channel RabbitMQChannelInterface) error {
	return channel.Qos(
		100,
		0,
		false,
	)
}

func ChannelConfirm(connection RabbitMQConnectionInterface, channel RabbitMQChannelInterface) error {
	return channel.Confirm(false)
}

func DialWrapper(url string) (RabbitMQConnectionInterface, error) {
	return amqp.Dial(url)
}
