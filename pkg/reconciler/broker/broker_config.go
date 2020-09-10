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

package broker

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"

	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
)

// This file contains the logic dealing with how to handle Broker.Spec.Config.

func (r *Reconciler) getExchangeArgs(ctx context.Context, b *v1beta1.Broker) (*resources.ExchangeArgs, error) {
	rabbitmqURL, err := r.rabbitmqURL(ctx, b)
	if err != nil {
		return nil, err
	}
	return &resources.ExchangeArgs{
		Broker:      b,
		RabbitMQURL: rabbitmqURL,
	}, nil
}

func (r *Reconciler) rabbitmqURL(ctx context.Context, b *v1beta1.Broker) (*url.URL, error) {
	if b.Spec.Config != nil {
		if b.Spec.Config.Namespace == "" || b.Spec.Config.Name == "" {
			return nil, errors.New("broker.spec.config.[name, namespace] are required") // TODO: throwing an error here is wrong. This should set a custom condition.
		}

		gvk := fmt.Sprintf("%s.%s", b.Spec.Config.Kind, b.Spec.Config.APIVersion)

		switch gvk {
		case "Secret.v1":
			u, err := r.rabbitmqURLFromSecret(ctx, b.Spec.Config)
			if err != nil {
				logging.FromContext(ctx).Errorw("Unable to load RabbitMQ Broker URL from Broker.Spec.Config as v1:Secret.", zap.Error(err))
			}
			return u, err // TODO: throwing an error here is wrong. This should set a custom condition.
		case "RabbitmqCluster.rabbitmq.com/v1beta1":
			u, err := r.rabbitmqURLFromRabbit(ctx, b.Spec.Config)
			if err != nil {
				logging.FromContext(ctx).Errorw("Unable to load RabbitMQ Broker URL from Broker.Spec.Config as rabbitmq.com/v1beta1:RabbitmqCluster.", zap.Error(err))
			}
			return u, err // TODO: throwing an error here is wrong. This should set a custom condition.
		default:
			return nil, errors.New("Broker.Spec.Config configuration not supported, only [kind: Secret, apiVersion: v1]") // TODO: throwing an error here is wrong. This should set a custom condition.
		}
	}
	return nil, errors.New("Broker.Spec.Config is required") // TODO: throwing an error here is wrong. This should set a custom condition.
}

func (r *Reconciler) rabbitmqURLFromSecret(ctx context.Context, ref *duckv1.KReference) (*url.URL, error) {
	s, err := r.kubeClientSet.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}
	val := s.Data[resources.BrokerURLSecretKey]
	if val == nil {
		return nil, fmt.Errorf("Secret missing key %s", resources.BrokerURLSecretKey)
	}

	return url.Parse(string(val))
}

func (r *Reconciler) rabbitmqURLFromRabbit(ctx context.Context, ref *duckv1.KReference) (*url.URL, error) {
	// TODO:
	return nil, errors.New("TODO: implement rabbitmqURLFromRabbit")
}
