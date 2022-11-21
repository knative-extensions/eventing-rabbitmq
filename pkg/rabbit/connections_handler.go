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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rabbitmq/amqp091-go"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type RabbitMQConnectionsHandlerInterface interface {
	GetConnection() RabbitMQConnectionInterface
	GetChannel() RabbitMQChannelInterface
	SetupRabbitMQConnectionAndChannel(context.Context, string, func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error, func(string) (RabbitMQConnectionWrapperInterface, error))
	WatchRabbitMQConnections(context.Context, string, func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error, func(string) (RabbitMQConnectionWrapperInterface, error))
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

func NewRabbitMQHelper(cycleDuration time.Duration, logger *zap.SugaredLogger) RabbitMQConnectionsHandlerInterface {
	return &RabbitMQHelper{
		firstSetup:    true,
		cycleDuration: cycleDuration,
		logger:        logger,
	}
}

func (r *RabbitMQHelper) SetupRabbitMQConnectionAndChannel(
	ctx context.Context,
	rabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error,
	dialFunc func(string) (RabbitMQConnectionWrapperInterface, error)) {
	var err error
	retryConnection, retryChannel := true, true
	for retryConnection || retryChannel {
		// Wait one cycle duration always except the first time
		if !r.firstSetup {
			time.Sleep(time.Second * r.cycleDuration)
		} else {
			r.logger.Info("Creating and Configuring RabbitMQ Connection and Channel")
			r.firstSetup = false
		}
		r.Connection, r.Channel, err = r.CreateAndConfigConnectionsAndChannel(&retryConnection, &retryChannel, rabbitMQURL, configFunction, dialFunc)
		if err != nil {
			r.logger.Error(err)
			if retryConnection {
				r.CloseRabbitMQConnections()
			}
		}
	}
	if r.firstSetup {
		go r.WatchRabbitMQConnections(ctx, rabbitMQURL, configFunction, dialFunc)
	}
}

func (r *RabbitMQHelper) WatchRabbitMQConnections(
	ctx context.Context,
	rabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error,
	dialFunc func(string) (RabbitMQConnectionWrapperInterface, error)) {
	defer r.CloseRabbitMQConnections()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("stopped watching for rabbitmq connections")
			return
		case <-r.GetConnection().NotifyClose(make(chan *amqp091.Error)):
		case <-r.GetChannel().NotifyClose(make(chan *amqp091.Error)):
		}
		r.CloseRabbitMQConnections()
		r.SetupRabbitMQConnectionAndChannel(ctx, rabbitMQURL, configFunction, dialFunc)
	}
}

func (r *RabbitMQHelper) CreateAndConfigConnectionsAndChannel(
	retryConnection, retryChannel *bool,
	rabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error,
	dialFunc func(string) (RabbitMQConnectionWrapperInterface, error),
) (RabbitMQConnectionWrapperInterface, RabbitMQChannelInterface, error) {
	var connection RabbitMQConnectionWrapperInterface
	var channel RabbitMQChannelInterface
	var err error
	// Create the connection
	if *retryConnection {
		if connection, err = dialFunc(rabbitMQURL); err != nil {
			err = fmt.Errorf("failed to create RabbitMQ connection, error: %w", err)
		} else {
			*retryConnection = false
		}
	}
	// Create the channel
	if err == nil && *retryChannel {
		if connection.IsClosed() {
			err, *retryConnection = amqp.ErrClosed, true
		} else {
			if channel, err = connection.ChannelWrapper(); err != nil {
				err = fmt.Errorf("failed to create RabbitMQ channel, error: %w", err)
			} else {
				*retryChannel = false
			}
		}
	}
	// Config the connection and channel if needed
	if err == nil && configFunction != nil {
		if err = configFunction(connection, channel); err != nil {
			err = fmt.Errorf("failed to configure RabbitMQ connections, error: %w", err)
			*retryConnection, *retryChannel = true, true
		}
	}

	return connection, channel, err
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
