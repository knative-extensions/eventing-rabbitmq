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
	"sync"
	"testing"

	wabbitamqp "github.com/NeowayLabs/wabbit/amqp"
	"github.com/NeowayLabs/wabbit/amqptest"
	"github.com/NeowayLabs/wabbit/amqptest/server"
	"go.uber.org/zap"
)

func Test_SetupRabbitMQ(t *testing.T) {
	retryChannel := make(chan bool)
	testChan := make(chan bool)
	logger := zap.NewNop().Sugar()

	fakeServer := server.NewServer("amqp://localhost:5672/%2f")
	err := fakeServer.Start()
	if err != nil {
		t.Errorf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	// To avoid retrying here we set the variable to false
	Retrying = false
	_, _, err = SetupRabbitMQ("amqp://localhost:5672/%2f", retryChannel, logger)
	if err == nil {
		t.Error("SetupRabbitMQ should fail with the default DialFunc in testing environments")
	}

	Retrying = true
	SetDialFunc(amqptest.Dial)
	conn, ch, err := SetupRabbitMQ("amqp://localhost:5672/%2f", retryChannel, logger)
	if err != nil {
		t.Errorf("Error while creating RabbitMQ resources %s", err)
	} else if conn.IsClosed() || ch.IsClosed() {
		t.Error("Connection or Channel closed after creating them")
	}

	// function to test channel comunication between processes
	go func(t *testing.T) {
		for {
			retry := <-retryChannel
			testChan <- retry
			if !retry {
				t.Log("retryChannel closed correctly")
				break
			} else {
				t.Log("connections closed correctly")
			}
		}
	}(t)

	CloseRabbitMQConnections(conn, ch, logger)
	// wait for a response from the notifyClose channel to the retryChannel test process
	<-testChan
	if !conn.IsClosed() || !ch.IsClosed() {
		t.Error("Connection or Channel not closed after close method")
	}

	CleanupRabbitMQ(conn, ch, retryChannel, logger)
	if !conn.IsClosed() || !ch.IsClosed() {
		t.Error("Connection or Channel not closed after cleanup method")
	}
	// wait for a response from the notifyClose channel to the retryChannel test process
	<-testChan
}

func Test_SetupExponentialBackoff(t *testing.T) {
	var wg sync.WaitGroup
	retryChannel := make(chan bool)
	logger := zap.NewNop().Sugar()
	// set Retrying to false to test only edge cases without retrying
	Retrying = false
	// SetDialFunc to one that always fails
	SetDialFunc(wabbitamqp.Dial)

	// use a non blocking thread running the retries and wait for it
	wg.Add(1)
	go func() {
		SetupRabbitMQ("amqp://localhost:5672/%2f", retryChannel, logger)
		wg.Done()
	}()
	wg.Wait()
	if retryCounter == 0 {
		t.Errorf("no retries have been attempted want: > 0, got: %d", retryCounter)
	}

	retryCounter = 60
	wg.Add(1)
	go func() {
		SetupRabbitMQ("amqp://localhost:5672/%2f", retryChannel, logger)
		wg.Done()
	}()
	wg.Wait()
	if cycleNumber == 0 || cycleDuration == 1 {
		t.Errorf("cycles haven't been updated want: 1 2, got: %d %d", cycleNumber, cycleDuration)
	}

	cycleNumber = 0
	cycleDuration = 2
	cycleExp := cycleDuration * cycleDuration
	retryCounter = 60
	wg.Add(1)
	go func() {
		SetupRabbitMQ("amqp://localhost:5672/%2f", retryChannel, logger)
		wg.Done()
	}()
	wg.Wait()
	if cycleDuration < cycleExp {
		t.Errorf("cycle duration is not been updated exponentially got: %d, want: %d", cycleDuration, cycleExp)
	}

	retryCounter = 60
	maxCycleDuration = 1
	wg.Add(1)
	go func() {
		SetupRabbitMQ("amqp://localhost:5672/%2f", retryChannel, logger)
		wg.Done()
	}()
	wg.Wait()
	if cycleDuration > maxCycleDuration {
		t.Errorf("cycleDuration is greater than maxCycleDuration %d", cycleDuration)
	}
}
