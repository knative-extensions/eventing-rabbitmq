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

	rabbitv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	brokerresources "knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/pkg/kmeta"

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

func NewBinding(ctx context.Context, broker *eventingv1.Broker, trigger *eventingv1.Trigger) (*rabbitv1beta1.Binding, error) {
	var or metav1.OwnerReference
	var bindingName string
	var bindingKey string
	var sourceName string

	arguments := map[string]string{
		"x-match": "all",
	}
	// Depending on if we have a Broker & Trigger we need to rejigger some of the names and
	// configurations little differently, so do that up front before actually creating the resource
	// that we're returning.
	if trigger != nil {
		or = *kmeta.NewControllerRef(trigger)
		bindingName = CreateTriggerQueueName(trigger)
		bindingKey = trigger.Name
		sourceName = brokerresources.ExchangeName(broker, false)
		if trigger.Spec.Filter != nil && trigger.Spec.Filter.Attributes != nil {
			for key, val := range trigger.Spec.Filter.Attributes {
				arguments[key] = val
			}
		}
	} else {
		or = *kmeta.NewControllerRef(broker)
		bindingName = CreateBrokerDeadLetterQueueName(broker)
		bindingKey = broker.Name
		sourceName = brokerresources.ExchangeName(broker, true)
	}
	arguments[BindingKey] = bindingKey

	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to encode binding arguments %+v : %s", argumentsJson, err)
	}

	binding := &rabbitv1beta1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       broker.Namespace,
			Name:            bindingName,
			OwnerReferences: []metav1.OwnerReference{or},
			Labels:          BindingLabels(broker, trigger),
		},
		Spec: rabbitv1beta1.BindingSpec{
			Vhost:           "/",
			Source:          sourceName,
			Destination:     bindingName,
			DestinationType: "queue",
			RoutingKey:      "",
			Arguments: &runtime.RawExtension{
				Raw: argumentsJson,
			},

			// TODO: We had before also internal / nowait set to false. Are these in Arguments,
			// or do they get sane defaults that we can just work with?
			// TODO: This one has to exist in the same namespace as this exchange.
			RabbitmqClusterReference: rabbitv1beta1.RabbitmqClusterReference{
				Name: broker.Spec.Config.Name,
			},
		},
	}
	return binding, nil
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
