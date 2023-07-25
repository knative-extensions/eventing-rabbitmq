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
	"fmt"

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/eventing-rabbitmq/pkg/brokerconfig"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/trigger/resources"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	rabbitv1beta1 "knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	rabbitclientset "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned"
	rabbitlisters "knative.dev/eventing-rabbitmq/third_party/pkg/client/listers/rabbitmq.com/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"knative.dev/eventing/pkg/duck"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

type Reconciler struct {
	eventingClientSet clientset.Interface
	kubeClientSet     kubernetes.Interface
	rabbitClientSet   rabbitclientset.Interface

	// listers index properties about resources
	deploymentLister appsv1listers.DeploymentLister
	brokerLister     eventinglisters.BrokerLister
	triggerLister    eventinglisters.TriggerLister
	exchangeLister   rabbitlisters.ExchangeLister
	queueLister      rabbitlisters.QueueLister
	bindingLister    rabbitlisters.BindingLister

	dispatcherImage              string
	dispatcherServiceAccountName string

	brokerClass string

	// Dynamic tracker to track KResources. In particular, it tracks the dependency between Triggers and Sources.
	sourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. In particular, it tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver
	// config accessor for observability/logging/tracing
	configs      reconcilersource.ConfigAccessor
	rabbit       rabbit.Service
	brokerConfig *brokerconfig.BrokerConfigService
}

// Check that our Reconciler implements Interface
var _ triggerreconciler.Interface = (*Reconciler)(nil)

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
	if broker.Annotations[eventing.BrokerClassKey] != r.brokerClass {
		logging.FromContext(ctx).Infof("Ignoring trigger %s/%s", t.Namespace, t.Name)
		return nil
	}

	t.Status.ObservedGeneration = t.Generation

	t.Status.PropagateBrokerCondition(broker.Status.GetTopLevelCondition())
	// If Broker is not ready, we're done, but once it becomes ready, we'll get requeued.
	if !broker.IsReady() {
		logging.FromContext(ctx).Errorw("Broker is not ready", zap.Any("Broker", broker))
		return nil
	}

	if err = r.checkDependencyAnnotation(ctx, t); err != nil {
		return err
	}

	// Just because there's no error, the dependency might not be ready, so check it.
	if ts := t.Status.GetCondition(eventingv1.TriggerConditionDependency); ts.Status != corev1.ConditionTrue {
		logging.FromContext(ctx).Info("Dependency is not ready")
		return nil
	}
	triggerQueueName := naming.CreateTriggerQueueName(t)

	var dlxName *string
	ref, err := r.brokerConfig.GetRabbitMQClusterRef(ctx, broker)
	if err != nil {
		return err
	}
	queueType, err := r.brokerConfig.GetQueueType(ctx, broker)
	if err != nil {
		return err
	}
	rabbitmqVhost, err := r.brokerConfig.GetRabbitMQVhost(ctx, broker)
	if err != nil {
		return err
	}
	if t.Spec.Delivery != nil && t.Spec.Delivery.DeadLetterSink != nil {
		// If there's DeadLetterSink, we need to create a DLX that's specific for this Trigger as well
		// as a Queue for it, and Dispatcher that pulls from that queue.
		dlxName = ptr.String(naming.TriggerDLXExchangeName(t))
		args := &rabbit.ExchangeArgs{
			Name:                     ptr.StringValue(dlxName),
			Namespace:                t.Namespace,
			Broker:                   broker,
			RabbitmqClusterReference: ref,
			RabbitMQVhost:            rabbitmqVhost,
			Trigger:                  t,
		}
		dlx, err := r.rabbit.ReconcileExchange(ctx, args)
		if err != nil {
			t.Status.MarkDependencyFailed("ExchangeFailure", fmt.Sprintf("Failed to reconcile DLX exchange %q: %s", naming.TriggerDLXExchangeName(t), err))
			return err
		}
		if !dlx.Ready {
			logging.FromContext(ctx).Warnf("DLX exchange %q is not ready", dlx.Name)
			t.Status.MarkDependencyFailed("ExchangeFailure", fmt.Sprintf("DLX exchange %q is not ready", dlx.Name))
			return nil
		}

		dlq, err := r.rabbit.ReconcileQueue(ctx, &rabbit.QueueArgs{
			Name:                     naming.CreateTriggerDeadLetterQueueName(t),
			Namespace:                t.Namespace,
			RabbitmqClusterReference: ref,
			RabbitMQVhost:            rabbitmqVhost,
			Owner:                    *kmeta.NewControllerRef(t),
			Labels:                   rabbit.Labels(broker, t, nil),
			QueueType:                queueType,
		})
		if err != nil {
			logging.FromContext(ctx).Error("Problem reconciling Trigger DLQ", zap.Error(err))
			t.Status.MarkDependencyFailed("DLQueueFailure", "%v", err)
			return err
		}
		if !dlq.Ready {
			logging.FromContext(ctx).Warnf("DLQ %q is not ready", dlq.Name)
			t.Status.MarkDependencyFailed("DLQueueFailure", "DLQ %q is not ready", dlq.Name)
			return nil
		}
		dlqBinding, err := r.reconcileDLQBinding(ctx, broker, t, rabbitmqVhost)
		if err != nil {
			logging.FromContext(ctx).Error("Problem reconciling Trigger DLQ Binding", zap.Error(err))
			t.Status.MarkDependencyFailed("BindingFailure", "%v", err)
			return err
		}
		if !dlqBinding.Ready {
			logging.FromContext(ctx).Warnf("DLQ Binding %q is not ready", dlqBinding.Name)
			t.Status.MarkDependencyFailed("BindingFailure", "DLQ Binding %q is not ready", dlqBinding.Name)
			return nil
		}
		deadLetterSinkURI, err := r.uriResolver.URIFromDestinationV1(ctx, *t.Spec.Delivery.DeadLetterSink, t)
		if err != nil {
			logging.FromContext(ctx).Error("Unable to get the DeadLetterSink URI", zap.Error(err))
			t.Status.MarkDeadLetterSinkResolvedFailed("Unable to get the DeadLetterSink URI", "%v", err)
			t.Status.DeadLetterSinkURI = nil
			return err
		}
		t.Status.MarkDeadLetterSinkResolvedSucceeded()
		t.Status.DeadLetterSinkURI = deadLetterSinkURI
		_, err = r.reconcileDispatcherDeployment(ctx, t, deadLetterSinkURI, t.Spec.Delivery, true, rabbitmqVhost)
		if err != nil {
			logging.FromContext(ctx).Error("Problem reconciling DLX dispatcher Deployment", zap.Error(err))
			t.Status.MarkDependencyFailed("DeploymentFailure", "%v", err)
			return err
		}
	} else {
		// Clean up any leftover resources from a previously configured DeadLetterSink
		r.deleteDLQResources(ctx, t)
		// There's no Delivery spec, so just mark is as there's no DeadLetterSink Configured for it.
		t.Status.MarkDeadLetterSinkNotConfigured()
	}

	queueArgs := &rabbit.QueueArgs{
		Name:                     triggerQueueName,
		Namespace:                t.Namespace,
		QueueName:                naming.CreateTriggerQueueRabbitName(t, string(broker.GetUID())),
		RabbitmqClusterReference: ref,
		RabbitMQVhost:            rabbitmqVhost,
		Owner:                    *kmeta.NewControllerRef(t),
		Labels:                   rabbit.Labels(broker, t, nil),
		DLXName:                  dlxName,
		QueueType:                queueType,
	}

	queue, err := r.rabbit.ReconcileQueue(ctx, queueArgs)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling Trigger Queue", zap.Error(err))
		t.Status.MarkDependencyFailed("QueueFailure", "%v", err)
		return err
	}
	if !queue.Ready {
		logging.FromContext(ctx).Warnf("Queue %q is not ready", queue.Name)
		t.Status.MarkDependencyFailed("QueueFailure", "Queue %q is not ready", queue.Name)
		return nil
	}

	logging.FromContext(ctx).Info("Reconciled rabbitmq queue", zap.Any("queue", queue))

	if dlxName != nil {
		dlqPolicy, err := r.rabbit.ReconcileDLQPolicy(ctx, queueArgs)
		if err != nil {
			logging.FromContext(ctx).Error("Problem reconciling Trigger DLQ Policy", zap.Error(err))
		}
		if !dlqPolicy.Ready {
			logging.FromContext(ctx).Warnf("DLQ Policy %q is not ready", dlqPolicy.Name)
			t.Status.MarkDependencyFailed("DLQPolicyFailure", "DLQ Polcy %q is not ready", dlqPolicy.Name)
			return nil
		}
	}

	binding, err := r.reconcileBinding(ctx, broker, t, rabbitmqVhost)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling Trigger Queue Binding", zap.Error(err))
		t.Status.MarkDependencyFailed("BindingFailure", "%v", err)
		return err
	}
	if !binding.Ready {
		logging.FromContext(ctx).Warnf("Binding %q is not ready", binding.Name)
		t.Status.MarkDependencyFailed("BindingFailure", "Binding %q is not ready", binding.Name)
		return nil
	}
	logging.FromContext(ctx).Info("Reconciled rabbitmq binding", zap.Any("binding", binding))
	t.Status.MarkDependencySucceeded()

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

	_, err = r.reconcileDispatcherDeployment(ctx, t, subscriberURI, delivery, false, rabbitmqVhost)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling dispatcher Deployment", zap.Error(err))
		t.Status.MarkDependencyFailed("DeploymentFailure", "%v", err)
		return err
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

func (r *Reconciler) deleteDLQResources(ctx context.Context, t *eventingv1.Trigger) {
	_ = r.rabbit.DeleteResource(ctx, &rabbit.DeleteResourceArgs{
		Kind:      rabbitv1beta1.Queue{},
		Name:      naming.CreateTriggerDeadLetterQueueName(t),
		Namespace: t.Namespace,
		Owner:     t,
	})

	_ = r.rabbit.DeleteResource(ctx, &rabbit.DeleteResourceArgs{
		Kind:      rabbitv1beta1.Exchange{},
		Name:      naming.TriggerDLXExchangeName(t),
		Namespace: t.Namespace,
		Owner:     t,
	})

	_ = r.rabbit.DeleteResource(ctx, &rabbit.DeleteResourceArgs{
		Kind:      rabbitv1beta1.Binding{},
		Name:      naming.CreateTriggerDeadLetterQueueName(t),
		Namespace: t.Namespace,
		Owner:     t,
	})

	_ = r.rabbit.DeleteResource(ctx, &rabbit.DeleteResourceArgs{
		Kind:      rabbitv1beta1.Policy{},
		Name:      naming.CreateTriggerQueueName(t),
		Namespace: t.Namespace,
		Owner:     t,
	})

	_ = r.deleteDeployment(ctx, t.Namespace, resources.DispatcherName(t.Name, true), t)
}

func (r *Reconciler) deleteDeployment(ctx context.Context, namespace, name string, t *eventingv1.Trigger) error {
	d, err := r.deploymentLister.Deployments(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	if !metav1.IsControlledBy(d, t) {
		return fmt.Errorf("deployment not owned by object: %v", t)
	}

	return r.kubeClientSet.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// reconcileDispatcherDeployment reconciles Trigger's dispatcher deployment.
func (r *Reconciler) reconcileDispatcherDeployment(
	ctx context.Context,
	t *eventingv1.Trigger,
	sub *apis.URL,
	delivery *eventingduckv1.DeliverySpec,
	dlq bool,
	rabbitmqVhost string) (*v1.Deployment, error) {
	rabbitmqSecret, err := r.getRabbitmqSecret(ctx, t)
	if err != nil {
		return nil, err
	}
	b, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		return nil, err
	}

	queueName := naming.CreateTriggerQueueRabbitName(t, string(b.GetUID()))
	if dlq {
		// overwrite to a dlq queueName if it's a dlq
		queueName = naming.CreateTriggerDeadLetterQueueName(t)
	}

	clusterRef, err := r.brokerConfig.GetRabbitMQClusterRef(ctx, b)
	if err != nil {
		return nil, err
	}
	secretName, err := r.rabbit.GetRabbitMQCASecret(ctx, clusterRef)
	if err != nil {
		return nil, err
	}
	resourceRequirements, err := utils.GetResourceRequirements(t.ObjectMeta)
	if err != nil {
		return nil, err
	}

	expected := resources.MakeDispatcherDeployment(&resources.DispatcherArgs{
		Trigger:              t,
		Image:                r.dispatcherImage,
		RabbitMQSecretName:   rabbitmqSecret.Name,
		RabbitMQCASecretName: secretName,
		RabbitMQVHost:        rabbitmqVhost,
		QueueName:            queueName,
		BrokerUrlSecretKey:   rabbit.BrokerURLSecretKey,
		BrokerIngressURL:     b.Status.Address.URL,
		Subscriber:           sub,
		DLX:                  dlq,
		Delivery:             delivery,
		Configs:              r.configs,
		ResourceRequirements: resourceRequirements,
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
	return r.kubeClientSet.CoreV1().Secrets(t.Namespace).Get(ctx, rabbit.SecretName(t.Spec.Broker, "broker"), metav1.GetOptions{})
}

func (r *Reconciler) reconcileBinding(ctx context.Context, b *eventingv1.Broker, t *eventingv1.Trigger, rabbitmqVhost string) (rabbit.Result, error) {
	bindingName := naming.CreateTriggerQueueName(t)
	var filters map[string]string
	if t.Spec.Filter != nil && t.Spec.Filter.Attributes != nil {
		filters = t.Spec.Filter.Attributes
	} else {
		filters = map[string]string{}
	}
	filters[rabbit.BindingKey] = t.Name
	ref, err := r.brokerConfig.GetRabbitMQClusterRef(ctx, b)
	if err != nil {
		return rabbit.Result{}, err
	}

	return r.rabbit.ReconcileBinding(ctx, &rabbit.BindingArgs{
		Name:                     bindingName,
		Namespace:                t.Namespace,
		RabbitmqClusterReference: ref,
		RabbitMQVhost:            rabbitmqVhost,
		Source:                   naming.BrokerExchangeName(b, false),
		Destination:              naming.CreateTriggerQueueRabbitName(t, string(b.GetUID())),
		Owner:                    *kmeta.NewControllerRef(t),
		Labels:                   rabbit.Labels(b, t, nil),
		Filters:                  filters,
	})
}

func (r *Reconciler) reconcileDLQBinding(ctx context.Context, b *eventingv1.Broker, t *eventingv1.Trigger, rabbitmqVhost string) (rabbit.Result, error) {
	bindingName := naming.CreateTriggerDeadLetterQueueName(t)
	ref, err := r.brokerConfig.GetRabbitMQClusterRef(ctx, b)
	if err != nil {
		return rabbit.Result{}, err
	}
	return r.rabbit.ReconcileBinding(ctx, &rabbit.BindingArgs{
		Name:                     bindingName,
		Namespace:                t.Namespace,
		RabbitmqClusterReference: ref,
		RabbitMQVhost:            rabbitmqVhost,
		Source:                   naming.TriggerDLXExchangeName(t),
		Destination:              bindingName,
		Owner:                    *kmeta.NewControllerRef(t),
		Labels:                   rabbit.Labels(b, t, nil),
	})
}
