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

	"github.com/NeowayLabs/wabbit"
	wabbitamqp "github.com/NeowayLabs/wabbit/amqp"
	"go.uber.org/zap"
)

const cycleRetries = 60

var (
	cycleNumber                                        = 0
	cycleDuration                                      = 1
	maxCycleDuration                                   = 7200
	DialFunc         func(string) (wabbit.Conn, error) = wabbitamqp.Dial
)

func SetDialFunc(dialFunc func(uri string) (wabbit.Conn, error)) {
	DialFunc = dialFunc
}

func SetupRabbitMQ(
	brokerURL string,
	retryNumber *int,
	retryChannel chan<- bool,
	logger *zap.SugaredLogger) (wabbit.Conn, wabbit.Channel, error) {
	if *retryNumber >= cycleRetries {
		cycleNumber += 1
		*retryNumber = 0
		if cycleDuration == 1 {
			cycleDuration += 1
		} else if cycleDuration >= maxCycleDuration {
			cycleDuration = maxCycleDuration
		} else {
			cycleDuration *= cycleDuration
		}

		logger.Warnf("Max retries (%d) reached for a cycle, adjusting duration to %ss", cycleRetries, cycleDuration)
	}

	var err error
	*retryNumber += 1
	conn, err := DialFunc(brokerURL)
	if err != nil {
		logger.Errorw("Failed to connect to RabbitMQ", zap.Error(err))
	}

	channel, err := conn.Channel()
	if err != nil {
		logger.Errorw("Failed to open a channel", zap.Error(err))
	}

	channel = channel.(*wabbitamqp.Channel)
	// noWait is false
	if err = channel.Confirm(false); err != nil {
		logger.Errorw("Failed to switch connection channel to confirm mode: %s", zap.Error(err))
	}

	// If there is an error trying to setup rabbit, retry
	if err != nil {
		time.Sleep(time.Second * time.Duration(cycleDuration))
		return SetupRabbitMQ(brokerURL, retryNumber, retryChannel, logger)
	}

	// If there is no error then, set the retries to zero and wait for a channel closed event again
	*retryNumber = 0
	// Wait for a channel or connection close message to rerun the RabbitMQ setup
	go WatchRabbitMQConnections(conn, channel, brokerURL, retryNumber, retryChannel, logger)
	return conn, channel, nil
}

func WatchRabbitMQConnections(
	connection wabbit.Conn,
	channel wabbit.Channel,
	brokerURL string,
	retryNumber *int,
	retryChannel chan<- bool,
	logger *zap.SugaredLogger) {
	var err error
	select {
	case err = <-connection.NotifyClose(make(chan wabbit.Error)):
	case err = <-channel.NotifyClose(make(chan wabbit.Error)):
	}
	logger.Warn(
		"Lost connection to RabbitMQ, reconnecting try number %d. Error: %v",
		*retryNumber+((cycleNumber+1)*cycleRetries),
		zap.Error(err))
	CloseRabbitMQConnections(connection, channel, logger)
	retryChannel <- true
}

func CloseRabbitMQConnections(connection wabbit.Conn, channel wabbit.Channel, logger *zap.SugaredLogger) {
	if err := channel.Close(); err != nil {
		logger.Error(err)
	}

	if err := connection.Close(); err != nil {
		logger.Error(err)
	}
}

func CleanupRabbitMQ(connection wabbit.Conn, channel wabbit.Channel, retryChannel chan<- bool, logger *zap.SugaredLogger) {
	retryChannel <- false
	close(retryChannel)
	CloseRabbitMQConnections(connection, channel, logger)
}
