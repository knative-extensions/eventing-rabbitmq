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
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	triggerresources "knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	rabbitclientset "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned"
	rabbitmqclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client"
	bindinginformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/binding"
	exchangeinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/exchange"
	queueinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/queue"
	rabbitlisters "knative.dev/eventing-rabbitmq/third_party/pkg/client/listers/rabbitmq.com/v1beta1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/logging"
)

func NewRabbit(ctx context.Context) *Rabbit {
	return &Rabbit{
		rabbitClientSet: rabbitmqclient.Get(ctx),
		exchangeLister:  exchangeinformer.Get(ctx).Lister(),
		queueLister:     queueinformer.Get(ctx).Lister(),
		bindingLister:   bindinginformer.Get(ctx).Lister(),
	}
}

func NewRabbitTest(
	rabbitClientSet rabbitclientset.Interface,
	exchangeLister rabbitlisters.ExchangeLister,
	queueLister rabbitlisters.QueueLister,
	bindingLister rabbitlisters.BindingLister,
) *Rabbit {
	// TODO(bmo): remove, use mocks
	return &Rabbit{
		rabbitClientSet: rabbitClientSet,
		exchangeLister:  exchangeLister,
		queueLister:     queueLister,
		bindingLister:   bindingLister,
	}
}

type Rabbit struct {
	rabbitClientSet rabbitclientset.Interface
	exchangeLister  rabbitlisters.ExchangeLister
	queueLister     rabbitlisters.QueueLister
	bindingLister   rabbitlisters.BindingLister
}

func (r *Rabbit) ReconcileExchange(ctx context.Context, args *resources.ExchangeArgs) (*v1beta1.Exchange, error) {
	logging.FromContext(ctx).Infow("Reconciling exchange", zap.String("name", args.Name))

	want := resources.NewExchange(ctx, args)
	current, err := r.exchangeLister.Exchanges(args.Broker.Namespace).Get(args.Name)
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

func (r *Rabbit) ReconcileQueue(ctx context.Context, args *triggerresources.QueueArgs) (*v1beta1.Queue, error) {
	logging.FromContext(ctx).Info("Reconciling queue")

	queueName := args.Name
	want := triggerresources.NewQueue(ctx, args)
	current, err := r.queueLister.Queues(args.Namespace).Get(queueName)
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq exchange", zap.String("queue name", want.Name))
		return r.rabbitClientSet.RabbitmqV1beta1().Queues(args.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		return r.rabbitClientSet.RabbitmqV1beta1().Queues(args.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	return current, nil
}

func (r *Rabbit) ReconcileBinding(ctx context.Context, b *eventingv1.Broker) (*v1beta1.Binding, error) {
	logging.FromContext(ctx).Info("Reconciling binding")

	// We can use the same name for queue / binding to keep things simpler
	bindingName := naming.CreateBrokerDeadLetterQueueName(b)

	want, err := triggerresources.NewBinding(ctx, b, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create the binding spec: %w", err)
	}
	current, err := r.bindingLister.Bindings(b.Namespace).Get(bindingName)
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Infow("Creating rabbitmq binding", zap.String("binding name", want.Name))
		return r.rabbitClientSet.RabbitmqV1beta1().Bindings(b.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		return r.rabbitClientSet.RabbitmqV1beta1().Bindings(b.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	return current, nil
}
