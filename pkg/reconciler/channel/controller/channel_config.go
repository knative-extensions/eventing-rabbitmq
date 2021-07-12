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

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/duck/v1beta1"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"

	"knative.dev/eventing-rabbitmq/pkg/reconciler/channel/controller/resources"
)

// This file contains the logic dealing with how to handle Broker.Spec.Config.

func (r *Reconciler) getExchangeArgs(ctx context.Context, c *messagingv1beta1.RabbitmqChannel) (*resources.ExchangeArgs, error) {
	rabbitmqURL, err := r.rabbitmqURL(ctx, c)
	if err != nil {
		return nil, err
	}
	return &resources.ExchangeArgs{
		Channel:     c,
		RabbitMQURL: rabbitmqURL,
	}, nil
}

func (r *Reconciler) rabbitmqURL(ctx context.Context, c *messagingv1beta1.RabbitmqChannel) (*url.URL, error) {
	if c.Spec.Config.Namespace == "" || c.Spec.Config.Name == "" {
		return nil, errors.New("rabbitmqchannel.spec.config.[name, namespace] are required")
	}

	gvk := fmt.Sprintf("%s.%s", c.Spec.Config.Kind, c.Spec.Config.APIVersion)

	switch gvk {
	case "Secret.v1":
		u, err := r.rabbitmqURLFromSecret(ctx, &c.Spec.Config)
		if err != nil {
			logging.FromContext(ctx).Errorw("Unable to load RabbitMQ Broker URL from Broker.Spec.Config as v1:Secret.", zap.Error(err))
		}
		return u, err
	case "RabbitmqCluster.rabbitmq.com/v1beta1":
		u, err := r.rabbitmqURLFromRabbit(ctx, &c.Spec.Config)
		if err != nil {
			logging.FromContext(ctx).Errorw("Unable to load RabbitMQ Broker URL from Broker.Spec.Config as rabbitmq.com/v1beta1:RabbitmqCluster.", zap.Error(err))
		}
		return u, err
	default:
		return nil, errors.New("Broker.Spec.Config configuration not supported, only [kind: Secret, apiVersion: v1 or kind: RabbitmqCluster, apiVersion: rabbitmq.com/v1beta1]")
	}

}

func (r *Reconciler) rabbitmqURLFromSecret(ctx context.Context, ref *duckv1.KReference) (*url.URL, error) {
	s, err := r.kubeClientSet.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}
	val := s.Data[resources.BrokerURLSecretKey]
	if val == nil {
		return nil, fmt.Errorf("secret missing key %s", resources.BrokerURLSecretKey)
	}

	return url.Parse(string(val))
}

func (r *Reconciler) rabbitmqURLFromRabbit(ctx context.Context, ref *duckv1.KReference) (*url.URL, error) {
	// TODO: make this better.

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
