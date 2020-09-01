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

package resources

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"reflect"

	rabbithole "github.com/michaelklishin/rabbit-hole"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
)

const (
	DefaultManagementPort = 15672
	BindingKey            = "x-knative-trigger"
)

// BindingArgs are the arguments to create a Trigger's Binding to a RabbitMQ Exchange.
type BindingArgs struct {
	Trigger                *eventingv1beta1.Trigger
	RoutingKey             string
	BrokerURL              string
	RabbitmqManagementPort int
}

// MakeBinding declares the Binding from the Broker's Exchange to the Trigger's Queue.
func MakeBinding(args *BindingArgs) error {
	uri, err := url.Parse(args.BrokerURL)
	if err != nil {
		return fmt.Errorf("failed to parse Broker URL: %v", err)
	}
	host, _, err := net.SplitHostPort(uri.Host)
	if err != nil {
		return fmt.Errorf("failed to resolve host from Broker URL: %v", err)
	}
	adminURL := fmt.Sprintf("http://%s:%d", host, managementPort(args))
	p, _ := uri.User.Password()
	c, err := rabbithole.NewClient(adminURL, uri.User.Username(), p)
	if err != nil {
		return fmt.Errorf("Failed to create RabbitMQ Admin Client: %v", err)
	}

	exchangeName := fmt.Sprintf("%s/%s", args.Trigger.Namespace, ExchangeName(args.Trigger.Spec.Broker))
	queueName := fmt.Sprintf("%s/%s", args.Trigger.Namespace, args.Trigger.Name)
	arguments := map[string]interface{}{
		"x-match":  interface{}("all"),
		BindingKey: interface{}(args.Trigger.Name),
	}
	for key, val := range args.Trigger.Spec.Filter.Attributes {
		arguments[key] = interface{}(val)
	}

	var existing *rabbithole.BindingInfo

	bindings, err := c.ListBindings()
	if err != nil {
		return err
	}

	for _, b := range bindings {
		if val, exists := b.Arguments[BindingKey]; exists && val == args.Trigger.Name {
			existing = &b
			break
		}
	}

	if existing == nil || !reflect.DeepEqual(existing.Arguments, arguments) {
		response, err := c.DeclareBinding("/", rabbithole.BindingInfo{
			Vhost:           "/",
			Source:          exchangeName,
			Destination:     queueName,
			DestinationType: "queue",
			RoutingKey:      args.RoutingKey,
			Arguments:       arguments,
		})
		if err != nil {
			return fmt.Errorf("Failed to declare Binding: %v", err)
		}
		if response.StatusCode != 201 {
			responseBody, _ := ioutil.ReadAll(response.Body)
			return fmt.Errorf("Failed to declare Binding. Expected 201 response, but got: %d.\n%s", response.StatusCode, string(responseBody))
		}
		if existing != nil {
			_, err = c.DeleteBinding(existing.Vhost, *existing)
			if err != nil {
				return fmt.Errorf("Failed to delete existing Binding: %v", err)
			}
		}
	}
	return nil
}

func managementPort(args *BindingArgs) int {
	configuredPort := args.RabbitmqManagementPort
	if configuredPort > 0 {
		return configuredPort
	}
	return DefaultManagementPort
}

// ExchangeName derives the Exchange name from the Broker name
func ExchangeName(brokerName string) string {
	return fmt.Sprintf("knative-%s", brokerName)
}
