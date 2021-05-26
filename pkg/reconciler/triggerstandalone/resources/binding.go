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

package resources

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"

	brokerresources "knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	DefaultManagementPort = 15672
	BindingKey            = "x-knative-trigger"
	DLQBindingKey         = "x-knative-dlq"
)

// BindingArgs are the arguments to create a Trigger's Binding to a RabbitMQ Exchange.
type BindingArgs struct {
	Trigger                *eventingv1.Trigger
	Broker                 *eventingv1.Broker // only for DLQ
	BindingName            string
	BindingKey             string
	RoutingKey             string
	SourceName             string
	BrokerURL              string
	RabbitmqManagementPort int
	AdminURL               string
	QueueName              string // only for DLQ
}

// BindingLabels generates the labels present on the Queue linking the Broker / Trigger to the
// Binding.
func BindingLabels(b *eventingv1.Broker, t *eventingv1.Trigger) map[string]string {
	if t == nil {
		return map[string]string{
			eventing.BrokerLabelKey: b.Name,
		}
	} else {
		return map[string]string{
			eventing.BrokerLabelKey: b.Name,
			TriggerLabelKey:         t.Name,
		}
	}
}

// MakeBinding declares the Binding from the Broker's Exchange to the Trigger's Queue.
func MakeBinding(transport http.RoundTripper, args *BindingArgs) error {
	uri, err := url.Parse(args.BrokerURL)
	if err != nil {
		return fmt.Errorf("failed to parse Broker URL: %v", err)
	}
	host, _, err := net.SplitHostPort(uri.Host)
	if err != nil {
		return fmt.Errorf("failed to resolve host from Broker URL: %v", err)
	}
	var adminURL string
	if args.AdminURL != "" {
		adminURL = args.AdminURL
	} else {
		adminURL = fmt.Sprintf("http://%s:%d", host, managementPort(args))
	}
	p, _ := uri.User.Password()
	c, err := rabbithole.NewClient(adminURL, uri.User.Username(), p)
	if err != nil {
		return fmt.Errorf("failed to create RabbitMQ Admin Client: %v", err)
	}
	if transport != nil {
		c.SetTransport(transport)
	}

	queueName := CreateTriggerQueueName(args.Trigger)
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
			Source:          brokerresources.ExchangeName(args.Broker, false),
			Destination:     queueName,
			DestinationType: "queue",
			RoutingKey:      args.RoutingKey,
			Arguments:       arguments,
		})
		if err != nil {
			return fmt.Errorf("failed to declare Binding: %v", err)
		}
		if response.StatusCode != 201 {
			responseBody, _ := ioutil.ReadAll(response.Body)
			return fmt.Errorf("failed to declare Binding. Expected 201 response, but got: %d.\n%s", response.StatusCode, string(responseBody))
		}
		if existing != nil {
			_, err = c.DeleteBinding(existing.Vhost, *existing)
			if err != nil {
				return fmt.Errorf("failed to delete existing Binding: %v", err)
			}
		}
	}
	return nil
}

// MakeDLQBinding declares the Binding from the Broker's DLX to the DLQ dispatchers Queue.
func MakeDLQBinding(transport http.RoundTripper, args *BindingArgs) error {
	uri, err := url.Parse(args.BrokerURL)
	if err != nil {
		return fmt.Errorf("failed to parse Broker URL: %v", err)
	}
	host, _, err := net.SplitHostPort(uri.Host)
	if err != nil {
		return fmt.Errorf("failed to resolve host from Broker URL: %v", err)
	}
	var adminURL string
	if args.AdminURL != "" {
		adminURL = args.AdminURL
	} else {
		adminURL = fmt.Sprintf("http://%s:%d", host, managementPort(args))
	}
	p, _ := uri.User.Password()
	c, err := rabbithole.NewClient(adminURL, uri.User.Username(), p)
	if err != nil {
		return fmt.Errorf("failed to create RabbitMQ Admin Client: %v", err)
	}
	if transport != nil {
		c.SetTransport(transport)
	}

	arguments := map[string]interface{}{
		"x-match":  interface{}("all"),
		BindingKey: interface{}(args.Broker.Name),
	}

	var existing *rabbithole.BindingInfo

	bindings, err := c.ListBindings()
	if err != nil {
		return err
	}

	for _, b := range bindings {
		if val, exists := b.Arguments[BindingKey]; exists && val == args.Broker.Name {
			existing = &b
			break
		}
	}

	if existing == nil || !reflect.DeepEqual(existing.Arguments, arguments) {
		response, err := c.DeclareBinding("/", rabbithole.BindingInfo{
			Vhost:           "/",
			Source:          brokerresources.ExchangeName(args.Broker, true),
			Destination:     args.QueueName,
			DestinationType: "queue",
			RoutingKey:      args.RoutingKey,
			Arguments:       arguments,
		})
		if err != nil {
			return fmt.Errorf("failed to declare DLQ Binding: %v", err)
		}
		if response.StatusCode != 201 {
			responseBody, _ := ioutil.ReadAll(response.Body)
			return fmt.Errorf("failed to declare Binding. Expected 201 response, but got: %d.\n%s", response.StatusCode, string(responseBody))
		}
		if existing != nil {
			_, err = c.DeleteBinding(existing.Vhost, *existing)
			if err != nil {
				return fmt.Errorf("failed to delete existing Binding: %v", err)
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
