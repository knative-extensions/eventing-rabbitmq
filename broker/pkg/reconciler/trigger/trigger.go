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

package trigger

import (
	"context"
	"errors"
	"fmt"

	"github.com/streadway/amqp"

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"

	"knative.dev/eventing-rabbitmq/broker/pkg/reconciler/broker"
	"knative.dev/eventing-rabbitmq/broker/pkg/reconciler/trigger/resources"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/trigger"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"

	"knative.dev/eventing/pkg/duck"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	kedaclientset "knative.dev/eventing-rabbitmq/broker/pkg/internal/thirdparty/keda/client/clientset/versioned"
	kedalisters "knative.dev/eventing-rabbitmq/broker/pkg/internal/thirdparty/keda/client/listers/keda/v1alpha1"
)

const (
	// Name of the corev1.Events emitted from the Trigger reconciliation process.
	triggerReconciled = "TriggerReconciled"
)

type Reconciler struct {
	kedaClientset     kedaclientset.Interface
	eventingClientSet clientset.Interface
	dynamicClientSet  dynamic.Interface
	kubeClientSet     kubernetes.Interface

	// listers index properties about resources
	deploymentLister   appsv1listers.DeploymentLister
	brokerLister       eventinglisters.BrokerLister
	triggerLister      eventinglisters.TriggerLister
	scaledObjectLister kedalisters.ScaledObjectLister

	dispatcherImage              string
	dispatcherServiceAccountName string

	// Dynamic tracker to track KResources. In particular, it tracks the dependency between Triggers and Sources.
	kresourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. In particular, it tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver
}

// Check that our Reconciler implements Interface
var _ triggerreconciler.Interface = (*Reconciler)(nil)
var _ triggerreconciler.Finalizer = (*Reconciler)(nil)

var triggerGVK = v1beta1.SchemeGroupVersion.WithKind("Trigger")

// ReconcilerArgs are the arguments needed to create a broker.Reconciler.
type ReconcilerArgs struct {
	DispatcherImage              string
	DispatcherServiceAccountName string
}

func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, triggerReconciled, "Trigger reconciled: \"%s/%s\"", namespace, name)
}

func (r *Reconciler) ReconcileKind(ctx context.Context, t *v1beta1.Trigger) pkgreconciler.Event {
	logging.FromContext(ctx).Debug("Reconciling", zap.Any("Trigger", t))
	t.Status.InitializeConditions()

	err := r.propagateBrokerStatus(ctx, t)
	if err != nil {
		return err
	}

	if err = r.checkDependencyAnnotation(ctx, t); err != nil {
		return err
	}

	t.Status.ObservedGeneration = t.Generation

	// 1. RabbitMQ Queue
	// 2. RabbitMQ Binding
	// 3. Dispatcher Deployment for Subscriber
	// 4. KEDA ScaledObject
	rabbitmqURL, err := r.rabbitmqURL(ctx, t)
	if err != nil {
		return err
	}

	queue, err := resources.DeclareQueue(&resources.QueueArgs{
		Trigger:     t,
		RabbitmqURL: rabbitmqURL,
	})
	if err != nil {
		logging.FromContext(ctx).Error("Problem declaring Trigger Queue", zap.Error(err))
		t.Status.MarkDependencyFailed("QueueFailure", "%v", err)
		return err
	}

	err = resources.MakeBinding(&resources.BindingArgs{
		Trigger:    t,
		RoutingKey: "",
		BrokerURL:  rabbitmqURL,
	})
	if err != nil {
		logging.FromContext(ctx).Error("Problem declaring Trigger Queue Binding", zap.Error(err))
		t.Status.MarkDependencyFailed("BindingFailure", "%v", err)
		return err
	}

	if t.Spec.Subscriber.Ref != nil {
		// To call URIFromDestination(dest apisv1alpha1.Destination, parent interface{}), dest.Ref must have a Namespace
		// We will use the Namespace of Trigger as the Namespace of dest.Ref
		t.Spec.Subscriber.Ref.Namespace = t.GetNamespace()
	}

	subscriberURI, err := r.uriResolver.URIFromDestinationV1(t.Spec.Subscriber, t)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Subscriber's URI", zap.Error(err))
		t.Status.MarkSubscriberResolvedFailed("Unable to get the Subscriber's URI", "%v", err)
		t.Status.SubscriberURI = nil
		return err
	}
	t.Status.SubscriberURI = subscriberURI
	t.Status.MarkSubscriberResolvedSucceeded()

	// TODO no Subscription
	t.Status.PropagateSubscriptionCondition(
		&apis.Condition{
			Type:   "Ready",
			Status: "True",
		},
	)

	deployment, err := r.reconcileDispatcherDeployment(ctx, t, subscriberURI)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling dispatcher Deployment", zap.Error(err))
		t.Status.MarkDependencyFailed("DeploymentFailure", "%v", err)
		return err
	}

	return r.reconcileScaledObject(queue, deployment, t)
}

func (r *Reconciler) FinalizeKind(ctx context.Context, t *v1beta1.Trigger) pkgreconciler.Event {
	rabbitmqURL, err := r.rabbitmqURL(ctx, t)
	if err != nil {
		return err
	}

	err = resources.DeleteQueue(&resources.QueueArgs{
		Trigger:     t,
		RabbitmqURL: rabbitmqURL,
	})
	if err != nil {
		return fmt.Errorf("Trigger finalize failed: %v", err)
	}
	return newReconciledNormal(t.Namespace, t.Name)
}

// reconcileDeployment reconciles the K8s Deployment 'd'.
func (r *Reconciler) reconcileDeployment(ctx context.Context, d *v1.Deployment) (*v1.Deployment, error) {
	current, err := r.deploymentLister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		_, err = r.kubeClientSet.AppsV1().Deployments(d.Namespace).Create(d)
		if err != nil {
			return nil, err
		}
		return d, nil
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(d.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		_, err = r.kubeClientSet.AppsV1().Deployments(desired.Namespace).Update(desired)
		if err != nil {
			return nil, err
		}
		return desired, nil
	}
	return current, nil
}

//reconcileDispatcherDeployment reconciles Trigger's dispatcher deployment.
func (r *Reconciler) reconcileDispatcherDeployment(ctx context.Context, t *v1beta1.Trigger, sub *apis.URL) (*v1.Deployment, error) {
	rabbitmqSecret, err := r.getRabbitmqSecret(ctx, t)
	if err != nil {
		return nil, err
	}
	b, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		return nil, err
	}
	expected := resources.MakeDispatcherDeployment(&resources.DispatcherArgs{
		Trigger: t,
		Image:   r.dispatcherImage,
		//ServiceAccountName string
		RabbitMQSecretName: rabbitmqSecret.Name,
		QueueName:          t.Name,
		BrokerUrlSecretKey: broker.BrokerUrlSecretKey,
		BrokerIngressURL:   b.Status.Address.URL,
		Subscriber:         sub,
	})
	return r.reconcileDeployment(ctx, expected)
}

func (r *Reconciler) propagateBrokerStatus(ctx context.Context, t *v1beta1.Trigger) error {
	broker, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		if apierrs.IsNotFound(err) {
			t.Status.MarkBrokerFailed("BrokerDoesNotExist", "Broker %q does not exist", t.Spec.Broker)
		} else {
			return fmt.Errorf("retrieving broker: %v", err)
		}
	} else {
		t.Status.PropagateBrokerCondition(broker.Status.GetTopLevelCondition())
	}
	return nil
}

func (r *Reconciler) checkDependencyAnnotation(ctx context.Context, t *v1beta1.Trigger) error {
	if dependencyAnnotation, ok := t.GetAnnotations()[v1beta1.DependencyAnnotation]; ok {
		dependencyObjRef, err := v1beta1.GetObjRefFromDependencyAnnotation(dependencyAnnotation)
		if err != nil {
			t.Status.MarkDependencyFailed("ReferenceError", "Unable to unmarshal objectReference from dependency annotation of trigger: %v", err)
			return fmt.Errorf("getting object ref from dependency annotation %q: %v", dependencyAnnotation, err)
		}
		trackKResource := r.kresourceTracker.TrackInNamespace(t)
		// Trigger and its dependent source are in the same namespace, we already did the validation in the webhook.
		if err := trackKResource(dependencyObjRef); err != nil {
			return fmt.Errorf("tracking dependency: %v", err)
		}
		if err := r.propagateDependencyReadiness(ctx, t, dependencyObjRef); err != nil {
			return fmt.Errorf("propagating dependency readiness: %v", err)
		}
	} else {
		t.Status.MarkDependencySucceeded()
	}
	return nil
}

func (r *Reconciler) propagateDependencyReadiness(ctx context.Context, t *v1beta1.Trigger, dependencyObjRef corev1.ObjectReference) error {
	lister, err := r.kresourceTracker.ListerFor(dependencyObjRef)
	if err != nil {
		t.Status.MarkDependencyUnknown("ListerDoesNotExist", "Failed to retrieve lister: %v", err)
		return fmt.Errorf("retrieving lister: %v", err)
	}
	dependencyObj, err := lister.ByNamespace(t.GetNamespace()).Get(dependencyObjRef.Name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			t.Status.MarkDependencyFailed("DependencyDoesNotExist", "Dependency does not exist: %v", err)
		} else {
			t.Status.MarkDependencyUnknown("DependencyGetFailed", "Failed to get dependency: %v", err)
		}
		return fmt.Errorf("getting the dependency: %v", err)
	}
	dependency := dependencyObj.(*duckv1.KResource)

	// The dependency hasn't yet reconciled our latest changes to
	// its desired state, so its conditions are outdated.
	if dependency.GetGeneration() != dependency.Status.ObservedGeneration {
		logging.FromContext(ctx).Info("The ObjectMeta Generation of dependency is not equal to the observedGeneration of status",
			zap.Any("objectMetaGeneration", dependency.GetGeneration()),
			zap.Any("statusObservedGeneration", dependency.Status.ObservedGeneration))
		t.Status.MarkDependencyUnknown("GenerationNotEqual", "The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", dependency.GetGeneration(), dependency.Status.ObservedGeneration)
		return nil
	}
	t.Status.PropagateDependencyStatus(dependency)
	return nil
}

func (r *Reconciler) getRabbitmqSecret(ctx context.Context, t *v1beta1.Trigger) (*corev1.Secret, error) {
	b, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		return nil, err
	}

	if b.Spec.Config != nil {
		if b.Spec.Config.Kind == "Secret" && b.Spec.Config.APIVersion == "v1" {
			if b.Spec.Config.Namespace == "" || b.Spec.Config.Name == "" {
				logging.FromContext(ctx).Error("Broker.Spec.Config name and namespace are required",
					zap.String("namespace", b.Namespace), zap.String("name", b.Name))
				return nil, errors.New("Broker.Spec.Config name and namespace are required")
			}
			s, err := r.kubeClientSet.CoreV1().Secrets(b.Spec.Config.Namespace).Get(b.Spec.Config.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return s, nil
		}
		return nil, errors.New("Broker.Spec.Config configuration not supported, only [kind: Secret, apiVersion: v1]")
	}
	return nil, errors.New("Broker.Spec.Config is required")
}

func (r *Reconciler) rabbitmqURL(ctx context.Context, t *v1beta1.Trigger) (string, error) {
	s, err := r.getRabbitmqSecret(ctx, t)
	if err != nil {
		return "", err
	}
	val := s.Data[broker.BrokerUrlSecretKey]
	if val == nil {
		return "", fmt.Errorf("Secret missing key %s", broker.BrokerUrlSecretKey)
	}
	return string(val), nil
}

func (r *Reconciler) reconcileScaledObject(queue *amqp.Queue, deployment *v1.Deployment, trigger *v1beta1.Trigger) error {
	so := resources.MakeDispatcherScaledObject(&resources.DispatcherScaledObjectArgs{
		DispatcherDeployment:      deployment,
		QueueName:                 queue.Name,
		Trigger:                   trigger,
		BrokerUrlSecretKey:        broker.BrokerUrlSecretKey,
		TriggerAuthenticationName: fmt.Sprintf("%s-trigger-auth", trigger.Spec.Broker),
	})

	current, err := r.scaledObjectLister.ScaledObjects(so.Namespace).Get(so.Name)
	if apierrs.IsNotFound(err) {
		_, err = r.kedaClientset.KedaV1alpha1().ScaledObjects(so.Namespace).Create(so)
		return err
	}
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepDerivative(so.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = so.Spec
		_, err = r.kedaClientset.KedaV1alpha1().ScaledObjects(desired.Namespace).Update(desired)
		return err
	}
	return nil
}
