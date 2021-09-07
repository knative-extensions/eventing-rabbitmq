/*
Copyright 2020 The Knative Authors

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

package amqp

import (
	"knative.dev/eventing-rabbitmq/wabbit"
	"knative.dev/eventing-rabbitmq/wabbit/amqp"
	"knative.dev/eventing-rabbitmq/wabbit/amqptest"
)

type DialerFunc func(rabbitURL string) (wabbit.Conn, error)

func RealDialer(rabbitURL string) (wabbit.Conn, error) {
	realCon, err := amqp.Dial(rabbitURL)
	return wabbit.Conn(realCon), err
}

func TestDialer(rabbitURL string) (wabbit.Conn, error) {
	fakeCon, err := amqptest.Dial(rabbitURL)
	return wabbit.Conn(fakeCon), err
}

type FakeFixedConnection struct {
	conn wabbit.Conn
}

func NewFakeFixedConnection(conn wabbit.Conn) FakeFixedConnection {
	return FakeFixedConnection{conn: conn}
}

func (f *FakeFixedConnection) TestFixedConnection(rabbitURL string) (wabbit.Conn, error) {
	return f.conn, nil
}
