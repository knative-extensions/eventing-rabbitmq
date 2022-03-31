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
	retryCounter                                        = 0
	cycleNumber                                         = 0
	InitCycleDuration                                   = 1
	cycleDuration                                       = 1
	maxCycleDuration                                    = 60
	retrying                                            = true
	cleaningUp                                          = false
	DialFunc          func(string) (wabbit.Conn, error) = wabbitamqp.Dial
)

func SetDialFunc(dialFunc func(uri string) (wabbit.Conn, error)) {
	DialFunc = dialFunc
}

func SetupRabbitMQ(
	RabbitMQURL string,
	retryChannel chan<- bool,
	logger *zap.SugaredLogger) (wabbit.Conn, wabbit.Channel, error) {
	// Calculate the current cycle time to sleep in seconds
	if retryCounter >= cycleRetries {
		cycleNumber += 1
		retryCounter = 0
		if cycleDuration == 1 {
			cycleDuration += 1
		} else if cycleDuration < maxCycleDuration {
			cycleDuration *= cycleDuration
		} else if cycleDuration > maxCycleDuration {
			cycleDuration = maxCycleDuration
		}
		logger.Warnf("Max retries (%d) reached for a cycle, adjusting duration to %ss", cycleRetries, cycleDuration)
	}

	retryCounter += 1
	var err error
	var conn wabbit.Conn
	var channel wabbit.Channel
	if conn, err = DialFunc(RabbitMQURL); err != nil {
		logger.Errorw("Failed to connect to RabbitMQ", zap.Error(err))
	} else if channel, err = conn.Channel(); err != nil {
		logger.Errorw("Failed to open a channel", zap.Error(err))
	}

	// If there is an error trying to setup rabbit, and the retrying is true retry
	if err != nil && retrying {
		time.Sleep(time.Second * time.Duration(cycleDuration*retryCounter))
		return SetupRabbitMQ(RabbitMQURL, retryChannel, logger)
	} else if err != nil && !retrying {
		return nil, nil, err
	}
	// if retrying is true and error nil reset the retryCounter and cycle values
	cycleDuration = InitCycleDuration
	cycleNumber = 0
	retryCounter = 0
	// Wait for a channel or connection close message to rerun the RabbitMQ setup
	go WatchRabbitMQConnections(conn, channel, RabbitMQURL, retryChannel, logger)
	return conn, channel, nil
}

func WatchRabbitMQConnections(
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
	if !cleaningUp {
		logger.Warn(
			"Lost connection to RabbitMQ, reconnecting retry number %d. Error: %v",
			retryCounter+((cycleNumber+1)*cycleRetries),
			zap.Error(err))
		CloseRabbitMQConnections(connection, channel, logger)
		cleaningUp = false
		retryChannel <- true
	}
}

func CloseRabbitMQConnections(connection wabbit.Conn, channel wabbit.Channel, logger *zap.SugaredLogger) {
	cleaningUp = true
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

func CleanupRabbitMQ(connection wabbit.Conn, channel wabbit.Channel, retryChannel chan<- bool, logger *zap.SugaredLogger) {
	retryChannel <- false
	CloseRabbitMQConnections(connection, channel, logger)
}
