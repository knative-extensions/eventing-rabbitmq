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

package services

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	rabbitclientset "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned"
	rabbitmqclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client"
	bindinginformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/binding"
	exchangeinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/exchange"
	queueinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/queue"
	rabbitlisters "knative.dev/eventing-rabbitmq/third_party/pkg/client/listers/rabbitmq.com/v1beta1"
	"knative.dev/pkg/logging"
)

func NewRabbit(ctx context.Context) *Rabbit {
	return &Rabbit{
		logger:          logging.FromContext(ctx),
		rabbitClientSet: rabbitmqclient.Get(ctx),
		exchangeLister:  exchangeinformer.Get(ctx).Lister(),
		queueLister:     queueinformer.Get(ctx).Lister(),
		bindingLister:   bindinginformer.Get(ctx).Lister(),
	}
}

func NewRabbitTest(
	logger *zap.SugaredLogger,
	rabbitClientSet rabbitclientset.Interface,
	exchangeLister rabbitlisters.ExchangeLister,
	queueLister rabbitlisters.QueueLister,
	bindingLister rabbitlisters.BindingLister,
) *Rabbit {
	// TODO(bmo): remove, use mocks
	return &Rabbit{
		logger:          logger,
		rabbitClientSet: rabbitClientSet,
		exchangeLister:  exchangeLister,
		queueLister:     queueLister,
		bindingLister:   bindingLister,
	}
}

type Rabbit struct {
	logger          *zap.SugaredLogger
	rabbitClientSet rabbitclientset.Interface
	exchangeLister  rabbitlisters.ExchangeLister
	queueLister     rabbitlisters.QueueLister
	bindingLister   rabbitlisters.BindingLister
}

func (r *Rabbit) ReconcileExchange(ctx context.Context, args *resources.ExchangeArgs) (*v1beta1.Exchange, error) {
	r.logger.Infow("Reconciling exchange", zap.Bool("dlx", args.DLX))

	want := resources.NewExchange(ctx, args)
	current, err := r.exchangeLister.Exchanges(args.Broker.Namespace).Get(naming.BrokerExchangeName(args.Broker, args.DLX))
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq exchange", zap.String("exchange name", want.Name))
		return r.rabbitClientSet.RabbitmqV1beta1().Exchanges(args.Broker.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		return r.rabbitClientSet.RabbitmqV1beta1().Exchanges(args.Broker.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	return current, nil
}
