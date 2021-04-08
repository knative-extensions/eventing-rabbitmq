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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"

	rabbitv1alpha2 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/kmeta"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
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

func NewBinding(ctx context.Context, broker *eventingv1.Broker, args *BindingArgs) (*rabbitv1alpha2.Binding, error) {
	arguments := map[string]string{
		"x-match":  "all",
		BindingKey: args.BindingKey,
	}
	if args.Trigger != nil && args.Trigger.Spec.Filter != nil {
		for key, val := range args.Trigger.Spec.Filter.Attributes {
			arguments[key] = val
		}
	}
	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to encode binding arguments %+v : %s", argumentsJson, err)
	}

	binding := &rabbitv1alpha2.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: broker.Namespace,
			Name:      args.BindingName,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(broker),
			},
			Labels: BindingLabels(broker),
		},
		Spec: rabbitv1alpha2.BindingSpec{
			Vhost:           "/",
			Source:          args.SourceName,
			Destination:     args.QueueName,
			DestinationType: "queue",
			RoutingKey:      args.RoutingKey,
			Arguments: &runtime.RawExtension{
				Raw: argumentsJson,
			},

			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			// TODO: This one has to exist in the same namespace as this exchange.
			RabbitmqClusterReference: rabbitv1alpha2.RabbitmqClusterReference{
				Name: broker.Spec.Config.Name,
			},
		},
	}
	return binding, nil
}

// BindingLabels generates the labels present on the Queue linking the Broker / Trigger to the
// Binding.
func BindingLabels(b *eventingv1.Broker) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker": b.Name,
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
		return fmt.Errorf("Failed to create RabbitMQ Admin Client: %v", err)
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
			Source:          ExchangeName(args.Trigger.Namespace, args.Trigger.Spec.Broker, false),
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
		return fmt.Errorf("Failed to create RabbitMQ Admin Client: %v", err)
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
			Source:          ExchangeName(args.Broker.Namespace, args.Broker.Name, true),
			Destination:     args.QueueName,
			DestinationType: "queue",
			RoutingKey:      args.RoutingKey,
			Arguments:       arguments,
		})
		if err != nil {
			return fmt.Errorf("Failed to declare DLQ Binding: %v", err)
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

func ExchangeName(namespace, brokerName string, DLX bool) string {
	if DLX {
		return fmt.Sprintf("%s.%s.dlx", namespace, brokerName)
	}
	return fmt.Sprintf("%s.%s", namespace, brokerName)
}
