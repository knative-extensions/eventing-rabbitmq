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
	GetConnection() RabbitMQConnectionInterface
	GetChannel() RabbitMQChannelInterface
	SetupRabbitMQConnectionAndChannel(string, func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error)
	CloseRabbitMQConnections()
}

type RabbitMQConnectionInterface interface {
	NotifyClose(chan *amqp.Error) chan *amqp.Error
	Close() error
	IsClosed() bool
}

type RabbitMQConnectionWrapperInterface interface {
	RabbitMQConnectionInterface
	ChannelWrapper() (RabbitMQChannelInterface, error)
}

type RabbitMQChannelInterface interface {
	IsClosed() bool
	NotifyClose(chan *amqp.Error) chan *amqp.Error
	Qos(int, int, bool) error
	Confirm(bool) error
	Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error)
	PublishWithDeferredConfirm(string, string, bool, bool, amqp.Publishing) (*amqp.DeferredConfirmation, error)
	QueueInspect(string) (amqp.Queue, error)
}

type RabbitMQHelper struct {
	firstSetup    bool
	cycleDuration time.Duration
	DialFunc      func(string) (RabbitMQConnectionWrapperInterface, error)
	Connection    RabbitMQConnectionWrapperInterface
	Channel       RabbitMQChannelInterface

	logger *zap.SugaredLogger
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
	} else if ci, ok := r.connection.(RabbitMQConnectionWrapperInterface); ok {
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
	logger *zap.SugaredLogger,
	dialFunc func(string) (RabbitMQConnectionWrapperInterface, error)) RabbitMQHelperInterface {
	return &RabbitMQHelper{
		cycleDuration: cycleDuration,
		logger:        logger,
		DialFunc:      dialFunc,
	}
}

func (r *RabbitMQHelper) SetupRabbitMQConnectionAndChannel(
	RabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error) {
	var err error
	var connInterface RabbitMQConnectionWrapperInterface
	var channelInterface RabbitMQChannelInterface
	retryConnection := true
	retryChannel := true

	for !retryConnection && !retryChannel {
		// Wait one cycle duration always except the first time
		time.Sleep(time.Second * r.cycleDuration)
		// Create the connection
		if retryConnection {
			if connInterface, err = r.DialFunc(RabbitMQURL); err != nil {
				r.logger.Errorw("failed to create RabbitMQ connection", zap.Error(err))
			} else {
				retryConnection = false
				r.Connection = connInterface
			}
		}
		// Create the channel
		if retryChannel {
			if connInterface.IsClosed() {
				err = amqp.ErrClosed
				retryConnection = true
			} else {
				if channelInterface, err = connInterface.ChannelWrapper(); err != nil {
					r.logger.Errorw("failed to create RabbitMQ channel", zap.Error(err))
				} else {
					retryChannel = false
					r.Channel = channelInterface
				}
			}
		}
		// Config the connection and channel if needed
		if err == nil && configFunction != nil {
			if err = configFunction(connInterface, channelInterface); err != nil {
				r.logger.Errorw("failed to configure RabbitMQ connections", zap.Error(err))
				retryConnection = true
				retryChannel = true
			}
		}
		// If there is an error trying to setup the connections wait and retry
		if err != nil {
			if retryConnection {
				r.CloseRabbitMQConnections()
				r.Connection = nil
			}
			r.Channel = nil
		}
	}
}

func (r *RabbitMQHelper) CloseRabbitMQConnections() {
	if r.Connection != nil && !r.Connection.IsClosed() {
		if err := r.Connection.Close(); err != nil { // This also close the associated channels
			r.logger.Error(err)
		}
	}
}

func (r *RabbitMQHelper) GetConnection() RabbitMQConnectionInterface {
	return r.Connection
}

func (r *RabbitMQHelper) GetChannel() RabbitMQChannelInterface {
	return r.Channel
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

func DialWrapper(url string) (RabbitMQConnectionWrapperInterface, error) {
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
