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
	"net/url"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	duckv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/duck/v1beta1"
	rabbitv1duck "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	rabbitclientset "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned"
	rabbitmqclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"
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

func (r *Rabbit) ReconcileExchange(ctx context.Context, args *ExchangeArgs) (Result, error) {
	logging.FromContext(ctx).Infow("Reconciling exchange", zap.String("name", args.Name))

	want := NewExchange(args)
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

func (r *Rabbit) ReconcileQueue(ctx context.Context, args *QueueArgs) (Result, error) {
	logging.FromContext(ctx).Infow("Reconciling queue", zap.String("name", args.Name))

	want := NewQueue(args)
	queue, err := r.RabbitmqV1beta1().Queues(args.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq queue", zap.String("queue", want.Name))
		queue, err = r.RabbitmqV1beta1().Queues(args.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return Result{}, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, queue.Spec) {
		// Don't modify the informers copy.
		desired := queue.DeepCopy()
		desired.Spec = want.Spec
		logging.FromContext(ctx).Debugw("Updating rabbitmq queue", zap.String("queue", queue.Name))
		queue, err = r.RabbitmqV1beta1().Queues(args.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	if err != nil {
		return Result{}, err
	}

	policyReady := true
	if args.DLXName != nil {
		want := NewPolicy(args)
		policy, err := r.RabbitmqV1beta1().Policies(args.Namespace).Get(ctx, queue.Name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Debugw("Creating rabbitmq policy", zap.String("name", queue.Name))
			policy, err = r.RabbitmqV1beta1().Policies(args.Namespace).Create(ctx, want, metav1.CreateOptions{})
		} else if err != nil {
			return Result{}, fmt.Errorf("error creating queue policy: %w", err)
		} else if !equality.Semantic.DeepDerivative(want.Spec, policy.Spec) {
			desired := policy.DeepCopy()
			desired.Spec = want.Spec
			logging.FromContext(ctx).Debugw("Updating rabbitmq policy", zap.String("name", queue.Name))
			policy, err = r.RabbitmqV1beta1().Policies(args.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
		}
		if err != nil {
			return Result{}, err
		}
		policyReady = isReady(policy.Status.Conditions)
	}
	return Result{
		Name:  want.Name,
		Ready: isReady(queue.Status.Conditions) && policyReady,
	}, nil
}

func (r *Rabbit) ReconcileBinding(ctx context.Context, args *BindingArgs) (Result, error) {
	logging.FromContext(ctx).Infow("Reconciling binding", zap.String("name", args.Name))

	want, err := NewBinding(args)
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

func (r *Rabbit) ReconcileBrokerDLXPolicy(ctx context.Context, args *QueueArgs) (Result, error) {
	logging.FromContext(ctx).Infow("Reconciling broker dead letter policy", zap.String("name", args.Name))

	want := NewBrokerDLXPolicy(args)
	policy, err := r.RabbitmqV1beta1().Policies(args.Namespace).Get(ctx, args.Name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq policy", zap.String("name", args.Name))
		policy, err = r.RabbitmqV1beta1().Policies(args.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return Result{}, fmt.Errorf("error creating rabbitmq policy: %w", err)
	} else if !equality.Semantic.DeepDerivative(want.Spec, policy.Spec) {
		desired := policy.DeepCopy()
		desired.Spec = want.Spec
		logging.FromContext(ctx).Debugw("Updating rabbitmq policy", zap.String("name", args.Name))
		policy, err = r.RabbitmqV1beta1().Policies(args.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	if err != nil {
		return Result{}, err
	}

	return Result{
		Name:  want.Name,
		Ready: isReady(policy.Status.Conditions),
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

func RabbitMQURL(ctx context.Context, clusterRef *rabbitv1beta1.RabbitmqClusterReference) (*url.URL, error) {
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
	informer := rabbitv1duck.Get(ctx)
	_, lister, err := informer.Get(ctx, gvr)
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

	s, err := kubeclient.Get(ctx).CoreV1().Secrets(rab.Status.DefaultUser.SecretReference.Namespace).Get(ctx, rab.Status.DefaultUser.SecretReference.Name, metav1.GetOptions{})
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
