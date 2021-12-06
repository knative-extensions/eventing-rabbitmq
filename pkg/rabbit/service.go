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

package rabbit

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	triggerresources "knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	rabbitclientset "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned"
	rabbitmqclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client"
	"knative.dev/pkg/logging"
)

func New(ctx context.Context) *Rabbit {
	return &Rabbit{
		Interface: rabbitmqclient.Get(ctx),
	}
}

var _ Service = (*Rabbit)(nil)

type Rabbit struct {
	rabbitclientset.Interface
}

func (r *Rabbit) ReconcileExchange(ctx context.Context, args *resources.ExchangeArgs) (Result, error) {
	logging.FromContext(ctx).Infow("Reconciling exchange", zap.String("name", args.Name))

	want := resources.NewExchange(ctx, args)
	current, err := r.RabbitmqV1beta1().Exchanges(args.Namespace).Get(ctx, args.Name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq exchange", zap.String("exchange name", want.Name))
		current, err = r.RabbitmqV1beta1().Exchanges(args.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return Result{}, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		current, err = r.RabbitmqV1beta1().Exchanges(args.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	if err != nil {
		return Result{}, err
	}
	return Result{
		Name:  want.Name,
		Ready: isReady(current.Status.Conditions),
	}, nil
}

func (r *Rabbit) ReconcileQueue(ctx context.Context, args *triggerresources.QueueArgs) (Result, error) {
	logging.FromContext(ctx).Info("Reconciling queue")

	queueName := args.Name
	want := triggerresources.NewQueue(ctx, args)
	current, err := r.RabbitmqV1beta1().Queues(args.Namespace).Get(ctx, queueName, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq exchange", zap.String("queue name", want.Name))
		current, err = r.RabbitmqV1beta1().Queues(args.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return Result{}, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		current, err = r.RabbitmqV1beta1().Queues(args.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	if err != nil {
		return Result{}, err
	}
	return Result{
		Name:  want.Name,
		Ready: isReady(current.Status.Conditions),
	}, nil
}

func (r *Rabbit) ReconcileBinding(ctx context.Context, args *triggerresources.BindingArgs) (Result, error) {
	logging.FromContext(ctx).Info("Reconciling binding")

	want, err := triggerresources.NewBinding(ctx, args)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create the binding spec: %w", err)
	}
	current, err := r.RabbitmqV1beta1().Bindings(args.Namespace).Get(ctx, args.Name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Infow("Creating rabbitmq binding", zap.String("binding name", want.Name))
		current, err = r.RabbitmqV1beta1().Bindings(args.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return Result{}, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		current, err = r.RabbitmqV1beta1().Bindings(args.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	if err != nil {
		return Result{}, err
	}
	return Result{
		Name:  want.Name,
		Ready: isReady(current.Status.Conditions),
	}, nil
}

func isReady(conditions []v1beta1.Condition) bool {
	numConditions := len(conditions)
	// If there are no conditions at all, the resource probably hasn't been reconciled yet => not ready
	if numConditions == 0 {
		return false
	}
	for _, c := range conditions {
		if c.Status == corev1.ConditionTrue {
			numConditions--
		}
	}
	return numConditions == 0
}
