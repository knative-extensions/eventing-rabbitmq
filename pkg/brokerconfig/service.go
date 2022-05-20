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
	"net/url"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	duckv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/duck/v1beta1"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/network"

	rmqeventingclientset "knative.dev/eventing-rabbitmq/pkg/client/clientset/versioned"
	rmqeventingclient "knative.dev/eventing-rabbitmq/pkg/client/injection/client"
	rabbitv1 "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	apisduck "knative.dev/pkg/apis/duck"
)

func New(ctx context.Context) *BrokerConfigService {
	return &BrokerConfigService{
		rmqeventingClientSet: rmqeventingclient.Get(ctx),
		kubeClientSet:        kubeclient.Get(ctx),
		rabbitLister:         rabbitv1.Get(ctx),
	}
}

type BrokerConfigService struct {
	rmqeventingClientSet rmqeventingclientset.Interface
	kubeClientSet        kubernetes.Interface
	rabbitLister         apisduck.InformerFactory
}

func (r *BrokerConfigService) GetExchangeArgs(ctx context.Context, b *eventingv1.Broker) (*rabbit.ExchangeArgs, error) {
	rabbitmqClusterRef, err := r.GetRabbitMQClusterRef(ctx, b)
	if err != nil {
		return nil, err
	}

	rabbitmqURL, err := r.RabbitmqURL(ctx, rabbitmqClusterRef)
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
		return nil, errors.New("Broker.Spec.Config configuration not supported, only [kind: RabbitmqCluster, apiVersion: rabbitmq.com/v1beta1]")
	}
}

func (r *BrokerConfigService) RabbitmqURL(ctx context.Context, clusterRef *rabbitv1beta1.RabbitmqClusterReference) (*url.URL, error) {
	// TODO: make this better.
	ref := &duckv1.KReference{
		Kind:       "RabbitmqCluster",
		APIVersion: "rabbitmq.com/v1beta1",
		Name:       clusterRef.Name,
		Namespace:  clusterRef.Namespace,
	}
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, err
	}
	gvk := gv.WithKind(ref.Kind)

	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	_, lister, err := r.rabbitLister.Get(ctx, gvr)
	if err != nil {
		return nil, err
	}

	o, err := lister.ByNamespace(ref.Namespace).Get(ref.Name)
	if err != nil {
		return nil, err
	}

	rab := o.(*duckv1beta1.Rabbit)

	if rab.Status.DefaultUser == nil || rab.Status.DefaultUser.SecretReference == nil || rab.Status.DefaultUser.ServiceReference == nil {
		return nil, fmt.Errorf("rabbit \"%s/%s\" not ready", ref.Namespace, ref.Name)
	}

	_ = rab.Status.DefaultUser.SecretReference

	s, err := r.kubeClientSet.CoreV1().Secrets(rab.Status.DefaultUser.SecretReference.Namespace).Get(ctx, rab.Status.DefaultUser.SecretReference.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	password, ok := s.Data[rab.Status.DefaultUser.SecretReference.Keys["password"]]
	if !ok {
		return nil, fmt.Errorf("rabbit Secret missing key %s", rab.Status.DefaultUser.SecretReference.Keys["password"])
	}

	username, ok := s.Data[rab.Status.DefaultUser.SecretReference.Keys["username"]]
	if !ok {
		return nil, fmt.Errorf("rabbit Secret missing key %s", rab.Status.DefaultUser.SecretReference.Keys["username"])
	}

	host := network.GetServiceHostname(rab.Status.DefaultUser.ServiceReference.Name, rab.Status.DefaultUser.ServiceReference.Namespace)

	return url.Parse(fmt.Sprintf("amqp://%s:%s@%s:%d", username, password, host, 5672))
}
