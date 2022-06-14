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

package brokerconfig

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"knative.dev/eventing-rabbitmq/pkg/apis/eventing/v1alpha1"
	rmqeventingclientset "knative.dev/eventing-rabbitmq/pkg/client/clientset/versioned"
	rmqeventingclient "knative.dev/eventing-rabbitmq/pkg/client/injection/client"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

func New(ctx context.Context) *BrokerConfigService {
	return &BrokerConfigService{
		rmqeventingClientSet: rmqeventingclient.Get(ctx),
		kubeClientSet:        kubeclient.Get(ctx),
		rabbitService:        rabbit.New(ctx),
	}
}

type BrokerConfigService struct {
	rmqeventingClientSet rmqeventingclientset.Interface
	kubeClientSet        kubernetes.Interface
	rabbitService        rabbit.Service
}

func (r *BrokerConfigService) GetExchangeArgs(ctx context.Context, b *eventingv1.Broker) (*rabbit.ExchangeArgs, error) {
	rabbitmqClusterRef, err := r.GetRabbitMQClusterRef(ctx, b)
	if err != nil {
		return nil, err
	}

	rabbitmqURL, err := r.rabbitService.RabbitMQURL(ctx, rabbitmqClusterRef, b.Namespace)
	if err != nil {
		return nil, err
	}

	return &rabbit.ExchangeArgs{
		Namespace:                b.Namespace,
		Broker:                   b,
		RabbitmqClusterReference: rabbitmqClusterRef,
		RabbitMQURL:              rabbitmqURL,
	}, nil
}

func (r *BrokerConfigService) GetRabbitMQClusterRef(ctx context.Context, b *eventingv1.Broker) (*rabbitv1beta1.RabbitmqClusterReference, error) {
	if b.Spec.Config == nil {
		return nil, errors.New("Broker.Spec.Config is required")
	}

	if b.Spec.Config.Namespace == "" || b.Spec.Config.Name == "" {
		return nil, errors.New("broker.spec.config.[name, namespace] are required")
	}

	gvk := fmt.Sprintf("%s.%s", b.Spec.Config.Kind, b.Spec.Config.APIVersion)
	switch gvk {
	case "RabbitmqCluster.rabbitmq.com/v1beta1":
		return &rabbitv1beta1.RabbitmqClusterReference{
			Name:      b.Spec.Config.Name,
			Namespace: b.Spec.Config.Namespace,
		}, nil
	case "RabbitmqBrokerConfig.eventing.knative.dev/v1alpha1":
		config, err := r.rmqeventingClientSet.EventingV1alpha1().RabbitmqBrokerConfigs(b.Spec.Config.Namespace).Get(ctx, b.Spec.Config.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if config.Spec.RabbitmqClusterReference == nil {
			return nil, fmt.Errorf("rabbitmqBrokerConfig %s/%s spec is empty", b.Spec.Config.Namespace, b.Spec.Config.Name)
		}
		return config.Spec.RabbitmqClusterReference, nil
	default:
		return nil, errors.New("Broker.Spec.Config configuration not supported, only [kind: RabbitmqCluster, apiVersion: rabbitmq.com/v1beta1] and [Kind: RabbitmqBrokerConfig, apiVersion: eventing.knative.dev/v1alpha1]")
	}
}

func (r *BrokerConfigService) GetQueueType(ctx context.Context, b *eventingv1.Broker) (v1alpha1.QueueType, error) {
	if b.Spec.Config == nil {
		return "", errors.New("Broker.Spec.Config is required")
	}

	if b.Spec.Config.Namespace == "" || b.Spec.Config.Name == "" {
		return "", errors.New("broker.spec.config.[name, namespace] are required")
	}

	gvk := fmt.Sprintf("%s.%s", b.Spec.Config.Kind, b.Spec.Config.APIVersion)
	switch gvk {
	case "RabbitmqCluster.rabbitmq.com/v1beta1":
		// Deprecated: returning the classic queue type for backwards compatibility of using RabbitmqCluster as a config
		return v1alpha1.ClassicQueueType, nil
	case "RabbitmqBrokerConfig.eventing.knative.dev/v1alpha1":
		config, err := r.rmqeventingClientSet.EventingV1alpha1().RabbitmqBrokerConfigs(b.Spec.Config.Namespace).Get(ctx, b.Spec.Config.Name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return config.Spec.QueueType, nil
	default:
		return "", errors.New("Broker.Spec.Config configuration not supported, only [kind: RabbitmqCluster, apiVersion: rabbitmq.com/v1beta1] and [Kind: RabbitmqBrokerConfig, apiVersion: eventing.knative.dev/v1alpha1]")
	}
}
