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

type RabbitMQHelper struct {
	retryCounter  int
	cycleDuration time.Duration
	cleaningUp    bool
}

func NewRabbitMQHelper(cycleDuration time.Duration) *RabbitMQHelper {
	return &RabbitMQHelper{
		cycleDuration: cycleDuration,
	}
}

func (r *RabbitMQHelper) SetupRabbitMQ(
	RabbitMQURL string,
	retryChannel chan<- bool,
	logger *zap.SugaredLogger) (*amqp.Connection, *amqp.Channel, error) {
	r.retryCounter += 1
	var err error
	var conn *amqp.Connection
	var channel *amqp.Channel
	if conn, err = amqp.Dial(RabbitMQURL); err != nil {
		logger.Errorw("failed to connect to RabbitMQ", zap.Error(err))
	} else if channel, err = conn.Channel(); err != nil {
		logger.Errorw("failed to open a channel", zap.Error(err))
	}

	// If there is an error trying to setup rabbit send a retry msg
	if err != nil {
		logger.Warnf("retry number %d", r.retryCounter)
		time.Sleep(time.Second * r.cycleDuration)
		go r.SignalRetry(retryChannel, true)
		return nil, nil, err
	}

	// if there is no error reset the retryCounter and cycle values
	r.retryCounter = 0
	// Wait for a channel or connection close message to rerun the RabbitMQ setup
	go r.WatchRabbitMQConnections(conn, channel, RabbitMQURL, retryChannel, logger)
	return conn, channel, nil
}

func (r *RabbitMQHelper) WatchRabbitMQConnections(
	connection *amqp.Connection,
	channel *amqp.Channel,
	RabbitMQURL string,
	retryChannel chan<- bool,
	logger *zap.SugaredLogger) {
	var err error
	select {
	case err = <-connection.NotifyClose(make(chan *amqp.Error)):
	case err = <-channel.NotifyClose(make(chan *amqp.Error)):
	}
	if !r.cleaningUp {
		logger.Warn(
			"Lost connection to RabbitMQ, reconnecting. Error: %v", zap.Error(err))
		r.CloseRabbitMQConnections(connection, channel, logger)
		r.SignalRetry(retryChannel, true)
	}
}

func (r *RabbitMQHelper) SignalRetry(retryChannel chan<- bool, retry bool) {
	retryChannel <- retry
}

func (r *RabbitMQHelper) CloseRabbitMQConnections(connection *amqp.Connection, channel *amqp.Channel, logger *zap.SugaredLogger) {
	r.cleaningUp = true
	if channel != nil && !channel.IsClosed() {
		if err := channel.Close(); err != nil {
			logger.Error(err)
		}
	}
	if connection != nil && !connection.IsClosed() {
		if err := connection.Close(); err != nil {
			logger.Error(err)
		}
	}
	r.cleaningUp = false
}

func (r *RabbitMQHelper) CleanupRabbitMQ(connection *amqp.Connection, channel *amqp.Channel, retryChannel chan<- bool, logger *zap.SugaredLogger) {
	r.SignalRetry(retryChannel, false)
	r.CloseRabbitMQConnections(connection, channel, logger)
}
