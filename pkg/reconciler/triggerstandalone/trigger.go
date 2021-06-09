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

package triggerstandalone

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"

	dialer "knative.dev/eventing-rabbitmq/pkg/amqp"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/triggerstandalone/resources"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"

	"knative.dev/eventing/pkg/duck"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	brokerresources "knative.dev/eventing-rabbitmq/pkg/reconciler/brokerstandalone/resources"
)

type Reconciler struct {
	eventingClientSet clientset.Interface
	dynamicClientSet  dynamic.Interface
	kubeClientSet     kubernetes.Interface

	// listers index properties about resources
	deploymentLister appsv1listers.DeploymentLister
	brokerLister     eventinglisters.BrokerLister
	triggerLister    eventinglisters.TriggerLister

	dispatcherImage              string
	dispatcherServiceAccountName string

	brokerClass string

	// Dynamic tracker to track KResources. In particular, it tracks the dependency between Triggers and Sources.
	sourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. In particular, it tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver

	// Which dialer to use.
	dialerFunc dialer.DialerFunc

	// Which HTTP transport to use
	transport http.RoundTripper
	// For testing...
	adminURL string
}

// Check that our Reconciler implements Interface
var _ triggerreconciler.Interface = (*Reconciler)(nil)
var _ triggerreconciler.Finalizer = (*Reconciler)(nil)

// isUsingOperator checks the Spec for a Broker and determines if we should be using the
// messaging-topology-operator or the libraries.
func isUsingOperator(b *eventingv1.Broker) bool {
	if b != nil && b.Spec.Config != nil {
		return b.Spec.Config.Kind == "RabbitmqCluster"
	}
	return false
}

func (r *Reconciler) ReconcileKind(ctx context.Context, t *eventingv1.Trigger) pkgreconciler.Event {
	logging.FromContext(ctx).Debug("Reconciling", zap.Any("Trigger", t))

	t.Status.InitializeConditions()

	broker, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		if apierrs.IsNotFound(err) {
			t.Status.MarkBrokerFailed("BrokerDoesNotExist", "Broker %q does not exist", t.Spec.Broker)
			// Ok to return nil here. Once the Broker comes available, or Trigger changes, we get requeued.
			return nil
		}
		t.Status.MarkBrokerFailed("FailedToGetBroker", "Failed to get broker %q : %s", t.Spec.Broker, err)
		return fmt.Errorf("retrieving broker: %v", err)
	}

	// If it's not my brokerclass, ignore
	// However, if for some reason it has my finalizer, remove it.
	// This is a bug in genreconciler because it slaps the finalizer in before we
	// know whether it is actually ours.
	if broker.Annotations[eventing.BrokerClassKey] != r.brokerClass {
		logging.FromContext(ctx).Infof("Ignoring trigger %s/%s", t.Namespace, t.Name)
		finalizers := sets.NewString(t.Finalizers...)
		if finalizers.Has(finalizerName) {
			finalizers.Delete(finalizerName)
			t.Finalizers = finalizers.List()
		}
		return nil
	}

	t.Status.ObservedGeneration = t.Generation

	if isUsingOperator(broker) {
		return fmt.Errorf("WON'T GO")
	}

	t.Status.PropagateBrokerCondition(broker.Status.GetTopLevelCondition())
	// If Broker is not ready, we're done, but once it becomes ready, we'll get requeued.
	if !broker.Status.IsReady() {
		logging.FromContext(ctx).Errorw("Broker is not ready", zap.Any("Broker", broker))
		return nil
	}

	if err = r.checkDependencyAnnotation(ctx, t); err != nil {
		return err
	}

	// 1. RabbitMQ Queue
	// 2. RabbitMQ Binding
	// 3. Dispatcher Deployment for Subscriber
	secretName, rabbitmqURL, err := r.rabbitmqURL(ctx, t)
	if err != nil {
		t.Status.MarkDependencyFailed("SecretFailure", "%v", err)
		return err
	}

	// By default use the Broker DLX, though it might get overridden below.
	dlxExchange := brokerresources.ExchangeName(broker, true)
	// If there's DeadLetterSink, we need to create a DLX that's specific for this Trigger as well
	// as a Queue for it, and Dispatcher that pulls from that queue.
	if t.Spec.Delivery != nil && t.Spec.Delivery.DeadLetterSink != nil {
		// TODO: We need the URL form here in different form when dealing with Exchanges
		exchangeURL, err := url.Parse(rabbitmqURL)
		if err != nil {
			return fmt.Errorf("failed to parse the URL for RabbitMQ: %s", err)
		}
		args := &brokerresources.ExchangeArgs{
			Broker:      broker,
			Trigger:     t,
			DLX:         true,
			RabbitMQURL: exchangeURL,
		}
		fmt.Printf("CREATING TRIGGER EXCHANGE: %+v\n", args)
		_, err = brokerresources.DeclareExchange(r.dialerFunc, args)
		if err != nil {
			t.Status.MarkDependencyFailed("ExchangeFailure", fmt.Sprintf("Failed to reconcile trigger DLX exchange %q: %s", brokerresources.TriggerDLXExchangeName(t), err))
			return err
		}
		// Use the Trigger specific exchange for the trigger queue (when created below).
		dlxExchange = brokerresources.TriggerDLXExchangeName(t)

		dlqArgs := &resources.QueueArgs{
			QueueName:   resources.CreateTriggerDeadLetterQueueName(t),
			RabbitmqURL: rabbitmqURL,
		}
		fmt.Printf("CREATING TRIGGER DLX QUEUE: %+v\n", dlqArgs)
		_, err = resources.DeclareQueue(r.dialerFunc, dlqArgs)
		if err != nil {
			logging.FromContext(ctx).Error("Problem reconciling Trigger Queue", zap.Error(err))
			t.Status.MarkDependencyFailed("QueueFailure", "%v", err)
			return err
		}

		fmt.Printf("CREATING TRIGGER DLX BINDING: %+v\n", dlqArgs)
		err = resources.MakeDLQBinding(r.transport, &resources.BindingArgs{
			Broker:     broker,
			Trigger:    t,
			RoutingKey: "",
			BrokerURL:  rabbitmqURL,
			AdminURL:   r.adminURL,
			QueueName:  resources.CreateTriggerDeadLetterQueueName(args.Trigger),
		})
		if err != nil {
			logging.FromContext(ctx).Error("Problem declaring Trigger DLQ Binding", zap.Error(err))
			t.Status.MarkDependencyFailed("BindingFailure", "%v", err)
			return err
		}
		deadLetterSinkURI, err := r.uriResolver.URIFromDestinationV1(ctx, *t.Spec.Delivery.DeadLetterSink, t)
		if err != nil {
			logging.FromContext(ctx).Error("Unable to get the DeadLetterSink URI", zap.Error(err))
			t.Status.MarkDependencyFailed("DeadLetterSink", fmt.Sprintf("Unable to get the DeadLetterSink URI: %s", err))
			return err
		}
		_, err = r.reconcileDLXDispatcherDeployment(ctx, broker, t, secretName, deadLetterSinkURI)
		if err != nil {
			logging.FromContext(ctx).Error("Problem reconciling DLX dispatcher Deployment", zap.Error(err))
			t.Status.MarkDependencyFailed("DeploymentFailure", "%v", err)
			return err
		}
	}

	// Note we use the same name for queue & binding for consistency.
	queueName := resources.CreateTriggerQueueName(t)
	queueArgs := &resources.QueueArgs{
		QueueName:   queueName,
		RabbitmqURL: rabbitmqURL,
		// This is either the Broker DLX or Queue DLX created above if Trigger has
		// delivery specified.
		DLX: dlxExchange,
	}

	queue, err := resources.DeclareQueue(r.dialerFunc, queueArgs)
	if err != nil {
		logging.FromContext(ctx).Error("Problem declaring Trigger Queue", zap.Error(err))
		t.Status.MarkDependencyFailed("QueueFailure", "%v", err)
		return err
	}
	logging.FromContext(ctx).Info("Created rabbitmq queue", zap.Any("queue", queue))

	err = resources.MakeBinding(r.transport, &resources.BindingArgs{
		Broker:     broker,
		Trigger:    t,
		RoutingKey: "",
		BrokerURL:  rabbitmqURL,
		AdminURL:   r.adminURL,
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

	subscriberURI, err := r.uriResolver.URIFromDestinationV1(ctx, t.Spec.Subscriber, t)
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

	// If trigger specified delivery, use it.
	delivery := t.Spec.Delivery
	if delivery == nil {
		// If trigger didn't but Broker did, use it instead.
		delivery = broker.Spec.Delivery
	}
	_, err = r.reconcileDispatcherDeployment(ctx, broker, t, secretName, subscriberURI, delivery)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling dispatcher Deployment", zap.Error(err))
		t.Status.MarkDependencyFailed("DeploymentFailure", "%v", err)
		return err
	}

	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, t *eventingv1.Trigger) pkgreconciler.Event {
	broker, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		if apierrs.IsNotFound(err) {
			// Ok to return nil here. There's nothing for us to do.
			return nil
		}
		return fmt.Errorf("retrieving broker: %v", err)
	}

	// If it's not my brokerclass, ignore
	// However, if for some reason it has my finalizer, remove it.
	// This is a bug in genreconciler because it slaps the finalizer in before we
	// know whether it is actually ours.
	if broker.Annotations[eventing.BrokerClassKey] != r.brokerClass {
		logging.FromContext(ctx).Infof("Ignoring trigger %s/%s", t.Namespace, t.Name)
		finalizers := sets.NewString(t.Finalizers...)
		if finalizers.Has(finalizerName) {
			finalizers.Delete(finalizerName)
			t.Finalizers = finalizers.List()
		}
		return nil
	}

	if isUsingOperator(broker) {
		// Everything gets cleaned up by garbage collection in this case.
		return nil
	}

	_, rabbitmqURL, err := r.rabbitmqURL(ctx, t)
	// If there's no secret, we can't delete the queue. Deleting an object should not require creation
	// of a secret, and for example if the namespace is being deleted, there's nothing we can do.
	// For now, return nil rather than leave the Trigger around.
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to fetch rabbitmq secret while finalizing, leaking a queue %s/%s", t.Namespace, t.Name)
		return nil
	}

	err = resources.DeleteQueue(r.dialerFunc, &resources.QueueArgs{
		QueueName:   resources.CreateTriggerQueueName(t),
		RabbitmqURL: rabbitmqURL,
	})
	if err != nil {
		return fmt.Errorf("trigger finalize failed: %v", err)
	}
	return nil
}

// reconcileDeployment reconciles the K8s Deployment 'd'.
func (r *Reconciler) reconcileDeployment(ctx context.Context, d *v1.Deployment) (*v1.Deployment, error) {
	current, err := r.deploymentLister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		_, err = r.kubeClientSet.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		return d, nil
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(d.Spec.Template, current.Spec.Template) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		_, err = r.kubeClientSet.AppsV1().Deployments(desired.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return desired, nil
	}
	return current, nil
}

//reconcileDispatcherDeployment reconciles Trigger's dispatcher deployment.
func (r *Reconciler) reconcileDispatcherDeployment(ctx context.Context, b *eventingv1.Broker, t *eventingv1.Trigger, secretName string, sub *apis.URL, delivery *eventingduckv1.DeliverySpec) (*v1.Deployment, error) {
	expected := resources.MakeDispatcherDeployment(&resources.DispatcherArgs{
		Trigger:            t,
		Image:              r.dispatcherImage,
		RabbitMQSecretName: secretName,
		QueueName:          resources.CreateTriggerQueueName(t),
		BrokerUrlSecretKey: brokerresources.BrokerURLSecretKey,
		BrokerIngressURL:   b.Status.Address.URL,
		Subscriber:         sub,
		Delivery:           delivery,
	})
	return r.reconcileDeployment(ctx, expected)
}

//reconcileDispatcherDeployment reconciles Trigger's dispatcher deployment.
func (r *Reconciler) reconcileDLXDispatcherDeployment(ctx context.Context, b *eventingv1.Broker, t *eventingv1.Trigger, secretName string, sub *apis.URL) (*v1.Deployment, error) {
	expected := resources.MakeDispatcherDeployment(&resources.DispatcherArgs{
		Trigger:            t,
		Image:              r.dispatcherImage,
		RabbitMQSecretName: secretName,
		QueueName:          resources.CreateTriggerDeadLetterQueueName(t),
		BrokerUrlSecretKey: brokerresources.BrokerURLSecretKey,
		BrokerIngressURL:   b.Status.Address.URL,
		Subscriber:         sub,
		DLX:                true,
	})
	return r.reconcileDeployment(ctx, expected)
}

func (r *Reconciler) checkDependencyAnnotation(ctx context.Context, t *eventingv1.Trigger) error {
	if dependencyAnnotation, ok := t.GetAnnotations()[eventingv1.DependencyAnnotation]; ok {
		dependencyObjRef, err := eventingv1.GetObjRefFromDependencyAnnotation(dependencyAnnotation)
		if err != nil {
			t.Status.MarkDependencyFailed("ReferenceError", "Unable to unmarshal objectReference from dependency annotation of trigger: %v", err)
			return fmt.Errorf("getting object ref from dependency annotation %q: %v", dependencyAnnotation, err)
		}
		trackKResource := r.sourceTracker.TrackInNamespace(ctx, t)
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

func (r *Reconciler) propagateDependencyReadiness(ctx context.Context, t *eventingv1.Trigger, dependencyObjRef corev1.ObjectReference) error {
	lister, err := r.sourceTracker.ListerFor(dependencyObjRef)
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
	dependency := dependencyObj.(*duckv1.Source)

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

func (r *Reconciler) getRabbitmqSecret(ctx context.Context, t *eventingv1.Trigger) (*corev1.Secret, error) {
	return r.kubeClientSet.CoreV1().Secrets(t.Namespace).Get(ctx, brokerresources.SecretName(t.Spec.Broker), metav1.GetOptions{})
}

// rabbitmqURL returns the URL for the Rabbit as well as the name of the secret to use
// to access it.
func (r *Reconciler) rabbitmqURL(ctx context.Context, t *eventingv1.Trigger) (string, string, error) {
	s, err := r.getRabbitmqSecret(ctx, t)
	if err != nil {
		return "", "", err
	}
	val := s.Data[brokerresources.BrokerURLSecretKey]
	if val == nil {
		return "", "", fmt.Errorf("secret missing key %s", brokerresources.BrokerURLSecretKey)
	}
	return s.Name, string(val), nil
}
