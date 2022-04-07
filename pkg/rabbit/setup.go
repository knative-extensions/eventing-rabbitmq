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

	"github.com/NeowayLabs/wabbit/amqp"

	"github.com/NeowayLabs/wabbit"
	"go.uber.org/zap"
)

type RabbitMQHelper struct {
	retryCounter  int
	cycleDuration time.Duration
	cleaningUp    bool
	dialFunc      func(string) (wabbit.Conn, error)
}

func NewRabbitMQHelper(cycleDuration time.Duration) *RabbitMQHelper {
	return &RabbitMQHelper{
		cycleDuration: cycleDuration,
		dialFunc:      amqp.Dial,
	}
}

func (r *RabbitMQHelper) SetDialFunc(dialFunc func(uri string) (wabbit.Conn, error)) {
	r.dialFunc = dialFunc
}

func (r *RabbitMQHelper) SetupRabbitMQ(
	RabbitMQURL string,
	retryChannel chan<- bool,
	logger *zap.SugaredLogger) (wabbit.Conn, wabbit.Channel, error) {
	r.retryCounter += 1
	var err error
	var conn wabbit.Conn
	var channel wabbit.Channel
	if conn, err = r.dialFunc(RabbitMQURL); err != nil {
		logger.Errorw("Failed to connect to RabbitMQ", zap.Error(err))
	} else if channel, err = conn.Channel(); err != nil {
		logger.Errorw("Failed to open a channel", zap.Error(err))
	}

	// If there is an error trying to setup rabbit send a retry msg
	if err != nil {
		time.Sleep(time.Second * r.cycleDuration)
		go func() { retryChannel <- true }()
		return nil, nil, err
	}

	// if there is no error reset the retryCounter and cycle values
	r.retryCounter = 0
	// Wait for a channel or connection close message to rerun the RabbitMQ setup
	go r.WatchRabbitMQConnections(conn, channel, RabbitMQURL, retryChannel, logger)
	return conn, channel, nil
}

func (r *RabbitMQHelper) WatchRabbitMQConnections(
	connection wabbit.Conn,
	channel wabbit.Channel,
	RabbitMQURL string,
	retryChannel chan<- bool,
	logger *zap.SugaredLogger) {
	var err error
	select {
	case err = <-connection.NotifyClose(make(chan wabbit.Error)):
	case err = <-channel.NotifyClose(make(chan wabbit.Error)):
	}
	if !r.cleaningUp {
		logger.Warn(
			"Lost connection to RabbitMQ, reconnecting retry number %d. Error: %v",
			r.retryCounter,
			zap.Error(err))
		r.CloseRabbitMQConnections(connection, channel, logger)
		r.cleaningUp = false
		retryChannel <- true
	}
}

func (r *RabbitMQHelper) CloseRabbitMQConnections(connection wabbit.Conn, channel wabbit.Channel, logger *zap.SugaredLogger) {
	r.cleaningUp = true
	if !channel.IsClosed() {
		if err := channel.Close(); err != nil {
			logger.Error(err)
		}
	}
	if !connection.IsClosed() {
		if err := connection.Close(); err != nil {
			logger.Error(err)
		}
	}
}

func (r *RabbitMQHelper) CleanupRabbitMQ(connection wabbit.Conn, channel wabbit.Channel, retryChannel chan<- bool, logger *zap.SugaredLogger) {
	retryChannel <- false
	r.CloseRabbitMQConnections(connection, channel, logger)
}
