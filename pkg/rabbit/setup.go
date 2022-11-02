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
	"fmt"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type RabbitMQHelperInterface interface {
	SetupRabbitMQ(string, func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error, *zap.SugaredLogger) (RabbitMQConnectionInterface, RabbitMQChannelInterface, error)
	WatchRabbitMQConnections(RabbitMQConnectionInterface, RabbitMQChannelInterface, *zap.SugaredLogger)
	SignalRetry(bool)
	WaitForRetrySignal() bool
	CloseRabbitMQConnections(RabbitMQConnectionInterface, *zap.SugaredLogger)
	CleanupRabbitMQ(connection RabbitMQConnectionInterface, logger *zap.SugaredLogger)
}

type RabbitMQConnectionInterface interface {
	NotifyClose(chan *amqp.Error) chan *amqp.Error
	Close() error
	IsClosed() bool
}

type RabbitMQChannelWrapperInterface interface {
	RabbitMQConnectionInterface
	ChannelWrapper() (RabbitMQChannelInterface, error)
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
	DialFunc      func(string) (RabbitMQChannelWrapperInterface, error)
}

type RabbitMQConnection struct {
	connection interface{}
}

func NewConnection(conn interface{}) *RabbitMQConnection {
	return &RabbitMQConnection{connection: conn}
}

func (r *RabbitMQConnection) ChannelWrapper() (RabbitMQChannelInterface, error) {
	if c, ok := r.connection.(*amqp.Connection); ok {
		return c.Channel()
	} else if ci, ok := r.connection.(RabbitMQChannelWrapperInterface); ok {
		return ci.ChannelWrapper()
	}
	return nil, errors.New("wrong typed connection arg")
}

func (r *RabbitMQConnection) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	if ci, ok := r.connection.(RabbitMQConnectionInterface); ok {
		return ci.NotifyClose(c)
	}
	close(c)
	return nil
}

func (r *RabbitMQConnection) Close() error {
	if ci, ok := r.connection.(RabbitMQConnectionInterface); ok {
		return ci.Close()
	}
	return errors.New("wrong typed connection arg")
}

func (r *RabbitMQConnection) IsClosed() bool {
	if ci, ok := r.connection.(RabbitMQConnectionInterface); ok {
		return ci.IsClosed()
	}
	return true
}

func NewRabbitMQHelper(
	cycleDuration time.Duration,
	retryChannel chan bool,
	dialFunc func(string) (RabbitMQChannelWrapperInterface, error)) RabbitMQHelperInterface {
	return &RabbitMQHelper{
		cycleDuration: cycleDuration,
		retryChannel:  retryChannel,
		DialFunc:      dialFunc,
	}
}

func (r *RabbitMQHelper) SetupRabbitMQ(
	RabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error,
	logger *zap.SugaredLogger) (RabbitMQConnectionInterface, RabbitMQChannelInterface, error) {
	r.retryCounter += 1
	var err error
	var connInterface RabbitMQChannelWrapperInterface
	var channelInterface RabbitMQChannelInterface
	if connInterface, err = r.DialFunc(RabbitMQURL); err != nil {
		logger.Errorw("failed to connect to RabbitMQ", zap.Error(err))
	} else if channelInterface, err = connInterface.ChannelWrapper(); err != nil {
		connInterface.Close()
		logger.Errorw("failed to open a RabbitMQ channel", zap.Error(err))
	}

	if configFunction != nil && err == nil {
		err = configFunction(connInterface, channelInterface)
	}

	// If there is an error trying to setup rabbit send a retry msg
	if err != nil {
		logger.Warnf("retry number %d", r.retryCounter)
		go r.SignalRetry(true)
		return nil, nil, err
	}

	// if there is no error reset the retryCounter and cycle values
	r.retryCounter = 0
	// Wait for a channel or connection close message to rerun the RabbitMQ setup
	go r.WatchRabbitMQConnections(connInterface, channelInterface, logger)
	return connInterface, channelInterface, nil
}

func (r *RabbitMQHelper) WatchRabbitMQConnections(
	connection RabbitMQConnectionInterface,
	channel RabbitMQChannelInterface,
	logger *zap.SugaredLogger) {
	var err error
	select {
	case err = <-connection.NotifyClose(make(chan *amqp.Error)):
	case err = <-channel.NotifyClose(make(chan *amqp.Error)):
	}
	if !r.cleaningUp {
		time.Sleep(time.Second * r.cycleDuration)
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

func DialWrapper(url string) (RabbitMQChannelWrapperInterface, error) {
	var rmqConn *RabbitMQConnection
	conn, err := amqp.Dial(url)
	if err == nil {
		rmqConn = NewConnection(conn)
	}
	return rmqConn, err
}

func VHostHandler(broker string, vhost string) string {
	if len(vhost) > 0 && len(broker) > 0 && !strings.HasSuffix(broker, "/") &&
		!strings.HasPrefix(vhost, "/") {
		return fmt.Sprintf("%s/%s", broker, vhost)
	}

	return fmt.Sprintf("%s%s", broker, vhost)
}
