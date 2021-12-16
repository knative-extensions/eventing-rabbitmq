/*
   Copyright 2021 The Knative Authors

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

package main

import (
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Channel struct {
	channel        *amqp.Channel
	counter        uint64
	mutex          sync.Mutex
	confirmsMap    sync.Map
	pconfirms_chan chan amqp.Confirmation
}

func openChannel(conn *amqp.Connection) *Channel {
	a_channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open a channel: %s", err)
	}

	channel := &Channel{channel: a_channel, counter: 1, pconfirms_chan: make(chan amqp.Confirmation)}

	go channel.confirmsHandler()

	a_channel.NotifyPublish(channel.pconfirms_chan)
	// noWait is false
	if err := a_channel.Confirm(false); err != nil {
		log.Fatalf("faild to switch connection channel to confirm mode: %s", err)
	}

	return channel
}

func (channel *Channel) publish(exchange string, headers amqp.Table, bytes []byte) (chan amqp.Confirmation, error) {
	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	cchan := make(chan amqp.Confirmation, 1)

	channel.confirmsMap.Store(channel.counter, cchan)

	if err := channel.channel.Publish(
		exchange,
		"",    // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:     headers,
			ContentType: "application/json",
			Body:        bytes,
		}); err != nil {
		channel.confirmsMap.LoadAndDelete(channel.counter)
		return nil, err
	}

	channel.counter++

	return cchan, nil
}

func (channel *Channel) confirmsHandler() func() error {
	return func() error {
		for confirm := range channel.pconfirms_chan {
			channel.handleConfirm(confirm)
		}
		return nil
	}
}

func (channel *Channel) handleConfirm(confirm amqp.Confirmation) {
	// NOTE: current golang driver resequences confirms and sends them one-by-one
	value, present := channel.confirmsMap.LoadAndDelete(confirm.DeliveryTag)

	if !present {
		// really die?
		log.Fatalf("Received confirm for unknown send. DeliveryTag: %d", confirm.DeliveryTag)
	}

	ch := value.(chan amqp.Confirmation)

	ch <- confirm
}

func (channel *Channel) Close() {
	// should I also stop confirmHandler here?
	// mb sending kill switch to channel.pconfirms_chan?
	channel.channel.Close()
}
