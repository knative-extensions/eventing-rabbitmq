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
	"go.uber.org/zap"
)

const reconnectionTriesThreshold = 6

type RabbitMQConnectionsHandlerInterface interface {
	GetConnection() RabbitMQConnectionInterface
	GetChannel() RabbitMQChannelInterface
	Setup(context.Context, string, func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error, func(string) (RabbitMQConnectionWrapperInterface, error))
}

type RabbitMQConnectionInterface interface {
	NotifyClose(chan *amqp091.Error) chan *amqp091.Error
	Close() error
	IsClosed() bool
}

type RabbitMQConnectionWrapperInterface interface {
	RabbitMQConnectionInterface
	ChannelWrapper() (RabbitMQChannelInterface, error)
}

type RabbitMQChannelInterface interface {
	IsClosed() bool
	NotifyClose(chan *amqp091.Error) chan *amqp091.Error
	Qos(int, int, bool) error
	Confirm(bool) error
	Consume(string, string, bool, bool, bool, bool, amqp091.Table) (<-chan amqp091.Delivery, error)
	PublishWithDeferredConfirm(string, string, bool, bool, amqp091.Publishing) (*amqp091.DeferredConfirmation, error)
	QueueDeclarePassive(string, bool, bool, bool, bool, amqp091.Table) (amqp091.Queue, error)
}

type RabbitMQConnectionHandler struct {
	firstSetup    bool
	reconTries    int
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
	if c, ok := r.connection.(*amqp091.Connection); ok {
		return c.Channel()
	} else if ci, ok := r.connection.(RabbitMQConnectionWrapperInterface); ok {
		return ci.ChannelWrapper()
	}
	return nil, errors.New("wrong typed connection arg")
}

func (r *RabbitMQConnection) NotifyClose(c chan *amqp091.Error) chan *amqp091.Error {
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

func NewRabbitMQConnectionHandler(cycleDuration time.Duration, logger *zap.SugaredLogger) RabbitMQConnectionsHandlerInterface {
	return &RabbitMQConnectionHandler{
		firstSetup:    true,
		cycleDuration: cycleDuration,
		logger:        logger,
	}
}

func (r *RabbitMQConnectionHandler) Setup(
	ctx context.Context,
	rabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error,
	dialFunc func(string) (RabbitMQConnectionWrapperInterface, error)) {
	r.createConnectionAndChannel(ctx, rabbitMQURL, configFunction, dialFunc)
	// watch for any connection unexpected closures
	if r.firstSetup {
		r.firstSetup = false
		go r.watchRabbitMQConnections(ctx, rabbitMQURL, configFunction, dialFunc)
	}
}

func (r *RabbitMQConnectionHandler) createConnectionAndChannel(
	ctx context.Context,
	rabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error,
	dialFunc func(string) (RabbitMQConnectionWrapperInterface, error)) {
	var err error
	for {
		// create the connection
		r.Connection, err = r.createConnection(rabbitMQURL, dialFunc)
		// create the channel
		if err == nil {
			r.Channel, err = r.createChannel()
		}
		// config channel and connection
		if err == nil && configFunction != nil {
			err = r.configConnectionAndChannel(configFunction)
		}
		// setup completed successfully
		if err == nil {
			r.reconTries = 0
			break
		}
		r.logger.Error(err)
		r.closeRabbitMQConnections()

		r.reconTries += 1
		if r.reconTries > reconnectionTriesThreshold {
			r.logger.Fatal("could not communicate to rabbitmq, restarting pods...")
		}
	}
}

func (r *RabbitMQConnectionHandler) watchRabbitMQConnections(
	ctx context.Context,
	rabbitMQURL string,
	configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error,
	dialFunc func(string) (RabbitMQConnectionWrapperInterface, error)) {
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("stopped watching for rabbitmq connections")
			r.closeRabbitMQConnections()
			return
		case <-r.GetConnection().NotifyClose(make(chan *amqp091.Error)):
		case <-r.GetChannel().NotifyClose(make(chan *amqp091.Error)):
		}
		r.closeRabbitMQConnections()
		r.createConnectionAndChannel(ctx, rabbitMQURL, configFunction, dialFunc)
	}
}

func (r *RabbitMQConnectionHandler) createConnection(rabbitMQURL string, dialFunc func(string) (RabbitMQConnectionWrapperInterface, error)) (RabbitMQConnectionWrapperInterface, error) {
	var connection RabbitMQConnectionWrapperInterface
	var err error
	// Create the connection
	if connection, err = dialFunc(rabbitMQURL); err != nil {
		return nil, fmt.Errorf("failed to create RabbitMQ connection, error: %w", err)
	}
	return connection, err
}

func (r *RabbitMQConnectionHandler) createChannel() (RabbitMQChannelInterface, error) {
	var channel RabbitMQChannelInterface
	var err error
	// Create the channel
	if r.Connection == nil || r.Connection.IsClosed() {
		return nil, amqp091.ErrClosed
	} else {
		if channel, err = r.Connection.ChannelWrapper(); err != nil {
			return nil, fmt.Errorf("failed to create RabbitMQ channel, error: %w", err)
		}
	}
	return channel, err
}

func (r *RabbitMQConnectionHandler) configConnectionAndChannel(configFunction func(RabbitMQConnectionInterface, RabbitMQChannelInterface) error) error {
	var err error
	// Config the connection and channel if needed
	if configFunction != nil {
		if err = configFunction(r.Connection, r.Channel); err != nil {
			return fmt.Errorf("failed to configure RabbitMQ connections, error: %w", err)
		}
	}
	return err
}

func (r *RabbitMQConnectionHandler) closeRabbitMQConnections() {
	if r.Connection != nil && !r.Connection.IsClosed() {
		if err := r.Connection.Close(); err != nil {
			r.logger.Error(err)
		}
	}
	// wait to prevent rabbitmq server from been spammed
	time.Sleep(time.Second)
}

func (r *RabbitMQConnectionHandler) GetConnection() RabbitMQConnectionInterface {
	return r.Connection
}

func (r *RabbitMQConnectionHandler) GetChannel() RabbitMQChannelInterface {
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
	conn, err := amqp091.Dial(url)
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
