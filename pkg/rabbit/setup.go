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
	"github.com/NeowayLabs/wabbit"
	wabbitamqp "github.com/NeowayLabs/wabbit/amqp"
	"go.uber.org/zap"
)

const maxRetries = 5

func SetupRabbitMQ(
	connection wabbit.Conn,
	channel wabbit.Channel,
	brokerURL string,
	retryNumber *int,
	logger *zap.SugaredLogger) {
	if *retryNumber >= maxRetries {
		logger.Fatalf("Max retries (%d) reached, unnable to setup RabbitMQ correctly", maxRetries)
		return
	}

	var err error
	*retryNumber += 1
	if connection == nil || connection.IsClosed() {
		connection, err = wabbitamqp.Dial(brokerURL)
		if err != nil {
			logger.Fatalw("Failed to connect to RabbitMQ", zap.Error(err))
		}
	}

	if channel, err = connection.Channel(); err != nil {
		logger.Fatalw("Failed to open a channel", zap.Error(err))
	}

	// noWait is false
	if err = channel.Confirm(false); err != nil {
		logger.Fatalf("Failed to switch connection channel to confirm mode: %s", err)
	}

	if err != nil {
		SetupRabbitMQ(connection, channel, brokerURL, retryNumber, logger)
		return
	}

	// Wait for a channel or connection close message to rerun the RabbitMQ setup
	go CleanupRabbitMQ(connection, channel, brokerURL, retryNumber, logger)
}

func CleanupRabbitMQ(
	connection wabbit.Conn,
	channel wabbit.Channel,
	brokerURL string,
	retryNumber *int,
	logger *zap.SugaredLogger) {
	var err error
	select {
	case err = <-connection.NotifyClose(make(chan wabbit.Error)):
	case err = <-channel.NotifyClose(make(chan wabbit.Error)):
	}
	logger.Fatalf(
		"Lost connection to RabbitMQ, reconnecting try number %d. Error: %v",
		*retryNumber,
		zap.Error(err))
	SetupRabbitMQ(connection, channel, brokerURL, retryNumber, logger)
}
