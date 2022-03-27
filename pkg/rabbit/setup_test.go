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
	"testing"

	"github.com/NeowayLabs/wabbit/amqptest"
	"github.com/NeowayLabs/wabbit/amqptest/server"
	"go.uber.org/zap"
)

func Test_DialFunc(t *testing.T) {
	if DialFunc == nil {
		t.Errorf("Default rabbit's pkg Dial function shouldn't be nil")
	}
}

func Test_SetupRabbitMQ(t *testing.T) {
	retryNumber := 0
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
	_, _, err = SetupRabbitMQ("amqp://localhost:5672/%2f", &retryNumber, retryChannel, logger)
	if err == nil {
		t.Error("SetupRabbitMQ should fail with the default DialFunc in testing environments")
	}

	Retrying = true
	SetDialFunc(amqptest.Dial)
	conn, ch, err := SetupRabbitMQ("amqp://localhost:5672/%2f", &retryNumber, retryChannel, logger)
	if err != nil {
		t.Errorf("Error while creating RabbitMQ resources %s", err)
	} else if conn.IsClosed() || ch.IsClosed() {
		t.Error("Connection or Channel closed after creating them")
	}

	// function to test channel comunication between processes
	go func(t *testing.T) {
		for {
			retry := <-retryChannel
			if !retry {
				t.Log("retryChannel closed correctly")
				testChan <- retry
				break
			}
		}
	}(t)

	CloseRabbitMQConnections(conn, ch, logger)
	if !conn.IsClosed() || !ch.IsClosed() {
		t.Error("Connection or Channel not closed after close method")
	}

	CleanupRabbitMQ(conn, ch, retryChannel, logger)
	if !conn.IsClosed() || !ch.IsClosed() {
		t.Error("Connection or Channel not closed after cleanup method")
	}
	// wait for a response from the retryChannel test process
	<-testChan
}
