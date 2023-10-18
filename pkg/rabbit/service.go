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
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	duckv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/duck/v1beta1"
	rabbitv1duck "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	rabbitclientset "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned"
	rabbitmqclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client"
	apisduck "knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"
)

const CA_SECRET_KEYNAME = "caSecretName"

func New(ctx context.Context) *Rabbit {
	return &Rabbit{
		Interface:     rabbitmqclient.Get(ctx),
		rabbitLister:  rabbitv1duck.Get(ctx),
		kubeClientSet: kubeclient.Get(ctx),
	}
}

var _ Service = (*Rabbit)(nil)

type Rabbit struct {
	rabbitclientset.Interface
	rabbitLister  apisduck.InformerFactory
	kubeClientSet kubernetes.Interface
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

	return Result{
		Name:  want.Name,
		Ready: isReady(queue.Status.Conditions),
	}, nil
}

func (r *Rabbit) ReconcileDLQPolicy(ctx context.Context, args *QueueArgs) (Result, error) {
	want := NewPolicy(args)
	policy, err := r.RabbitmqV1beta1().Policies(args.Namespace).Get(ctx, args.Name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq policy", zap.String("name", args.Name))
		policy, err = r.RabbitmqV1beta1().Policies(args.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return Result{}, fmt.Errorf("error creating queue policy: %w", err)
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

type DeleteResourceArgs struct {
	Kind      interface{}
	Name      string
	Namespace string
	Owner     metav1.Object
}

func (r *Rabbit) DeleteResource(ctx context.Context, args *DeleteResourceArgs) error {
	var o metav1.Object
	var err error

	switch args.Kind.(type) {
	case rabbitv1beta1.Queue:
		o, err = r.RabbitmqV1beta1().Queues(args.Namespace).Get(ctx, args.Name, metav1.GetOptions{})
	case rabbitv1beta1.Exchange:
		o, err = r.RabbitmqV1beta1().Exchanges(args.Namespace).Get(ctx, args.Name, metav1.GetOptions{})
	case rabbitv1beta1.Binding:
		o, err = r.RabbitmqV1beta1().Bindings(args.Namespace).Get(ctx, args.Name, metav1.GetOptions{})
	case rabbitv1beta1.Policy:
		o, err = r.RabbitmqV1beta1().Policies(args.Namespace).Get(ctx, args.Name, metav1.GetOptions{})
	default:
		return errors.New("unsupported type")
	}

	if apierrs.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	if !metav1.IsControlledBy(o, args.Owner) {
		return fmt.Errorf("%s not owned by object: %v", o.GetResourceVersion(), args.Owner)
	}

	switch args.Kind.(type) {
	case rabbitv1beta1.Queue:
		return r.RabbitmqV1beta1().Queues(args.Namespace).Delete(ctx, args.Name, metav1.DeleteOptions{})
	case rabbitv1beta1.Exchange:
		return r.RabbitmqV1beta1().Exchanges(args.Namespace).Delete(ctx, args.Name, metav1.DeleteOptions{})
	case rabbitv1beta1.Binding:
		return r.RabbitmqV1beta1().Bindings(args.Namespace).Delete(ctx, args.Name, metav1.DeleteOptions{})
	case rabbitv1beta1.Policy:
		return r.RabbitmqV1beta1().Policies(args.Namespace).Delete(ctx, args.Name, metav1.DeleteOptions{})
	}

	return nil
}

func isReady(conditions []rabbitv1beta1.Condition) bool {
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

func (r *Rabbit) RabbitMQURL(ctx context.Context, clusterRef *rabbitv1beta1.RabbitmqClusterReference) (string, error) {
	protocol := []byte("amqp")
	if clusterRef.ConnectionSecret != nil {
		s, err := r.kubeClientSet.CoreV1().Secrets(clusterRef.Namespace).Get(ctx, clusterRef.ConnectionSecret.Name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		uri, ok := s.Data["uri"]
		if !ok {
			return "", fmt.Errorf("rabbit Secret missing key uri")
		}
		uriString := string(uri)
		password, ok := s.Data["password"]
		if !ok {
			return "", fmt.Errorf("rabbit Secret missing key password")
		}
		username, ok := s.Data["username"]
		if !ok {
			return "", fmt.Errorf("rabbit Secret missing key username")
		}
		port, ok := s.Data["port"]
		if !ok {
			port = []byte("5672")
		}

		prefix := "http://"
		if strings.HasPrefix(strings.ToLower(uriString), "https") {
			protocol = []byte("amqps")
			prefix = "https://"
		}
		uriString = strings.TrimPrefix(uriString, prefix)
		splittedUri := strings.Split(uriString, ":")
		return fmt.Sprintf("%s://%s:%s@%s:%s", protocol, username, password, splittedUri[0], port), nil
	}

	rab, err := r.getClusterFromReference(ctx, clusterRef)
	if err != nil {
		return "", err
	}

	if rab.Status.DefaultUser == nil || rab.Status.DefaultUser.SecretReference == nil || rab.Status.DefaultUser.ServiceReference == nil {
		return "", fmt.Errorf("rabbit \"%s/%s\" not ready", rab.Namespace, rab.Name)
	}

	_ = rab.Status.DefaultUser.SecretReference

	s, err := r.kubeClientSet.CoreV1().Secrets(rab.Status.DefaultUser.SecretReference.Namespace).Get(ctx, rab.Status.DefaultUser.SecretReference.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	password, ok := s.Data[rab.Status.DefaultUser.SecretReference.Keys["password"]]
	if !ok {
		return "", fmt.Errorf("rabbit Secret missing key %s", rab.Status.DefaultUser.SecretReference.Keys["password"])
	}
	username, ok := s.Data[rab.Status.DefaultUser.SecretReference.Keys["username"]]
	if !ok {
		return "", fmt.Errorf("rabbit Secret missing key %s", rab.Status.DefaultUser.SecretReference.Keys["username"])
	}
	port, ok := s.Data["port"]
	if !ok {
		port = []byte("5672")
	}
	if (rab.Spec.TLS != nil && *rab.Spec.TLS != duckv1beta1.RabbitTLSConfig{}) {
		protocol = []byte("amqps")
	}
	host := network.GetServiceHostname(rab.Status.DefaultUser.ServiceReference.Name, rab.Status.DefaultUser.ServiceReference.Namespace)
	return fmt.Sprintf("%s://%s:%s@%s:%s", protocol, username, password, host, port), nil
}

func (r *Rabbit) GetRabbitMQCASecret(ctx context.Context, clusterRef *rabbitv1beta1.RabbitmqClusterReference) (string, error) {
	if clusterRef == nil {
		return "", errors.New("GetRabbitMQCASecret: nil clusterReference")
	}
	if clusterRef.ConnectionSecret != nil {
		s, err := r.kubeClientSet.CoreV1().Secrets(clusterRef.Namespace).Get(ctx, clusterRef.ConnectionSecret.Name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return string(s.Data[CA_SECRET_KEYNAME]), nil
	}

	rabbitMQCluster, err := r.getClusterFromReference(ctx, clusterRef)
	if err != nil {
		return "", err
	}
	if rabbitMQCluster.Spec.TLS != nil {
		return rabbitMQCluster.Spec.TLS.CASecretName, nil
	}

	return "", nil
}

func (r *Rabbit) getClusterFromReference(ctx context.Context, clusterRef *rabbitv1beta1.RabbitmqClusterReference) (*duckv1beta1.Rabbit, error) {
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
	return o.(*duckv1beta1.Rabbit), nil
}
