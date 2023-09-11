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
	"fmt"

	"k8s.io/utils/pointer"

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"

	"knative.dev/eventing-rabbitmq/pkg/brokerconfig"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	rabbitclientset "knative.dev/eventing-rabbitmq/third_party/pkg/client/clientset/versioned"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/duck"
	apisduck "knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

type Reconciler struct {
	eventingClientSet clientset.Interface
	kubeClientSet     kubernetes.Interface
	rabbitClientSet   rabbitclientset.Interface

	// listers index properties about resources
	brokerLister     eventinglisters.BrokerLister
	serviceLister    corev1listers.ServiceLister
	endpointsLister  corev1listers.EndpointsLister
	secretLister     corev1listers.SecretLister
	deploymentLister appsv1listers.DeploymentLister
	rabbitLister     apisduck.InformerFactory

	ingressImage              string
	ingressServiceAccountName string

	// Dynamic tracker to track KResources. In particular, it tracks the dependency between Triggers and Sources.
	kresourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. In particular, it tracks DLX sinks.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver

	// If specified, only reconcile brokers with these labels
	brokerClass string

	// Image to use for the DeadLetterSink dispatcher
	dispatcherImage string

	rabbit rabbit.Service
	// config accessor for observability/logging/tracing
	configs      reconcilersource.ConfigAccessor
	brokerConfig *brokerconfig.BrokerConfigService
}

// Check that our Reconciler implements Interface
var _ brokerreconciler.Interface = (*Reconciler)(nil)

// ReconcilerArgs are the arguments needed to create a broker.Reconciler.
type ReconcilerArgs struct {
	IngressImage              string
	IngressServiceAccountName string
}

const (
	BrokerConditionExchange       apis.ConditionType = "ExchangeReady"
	BrokerConditionDLX            apis.ConditionType = "DLXReady"
	BrokerConditionDeadLetterSink apis.ConditionType = "DeadLetterSinkReady"
	BrokerConditionSecret         apis.ConditionType = "SecretReady"
	BrokerConditionIngress        apis.ConditionType = "IngressReady"
	BrokerConditionAddressable    apis.ConditionType = "Addressable"
)

var rabbitBrokerCondSet = apis.NewLivingConditionSet(
	BrokerConditionExchange,
	BrokerConditionDLX,
	BrokerConditionDeadLetterSink,
	BrokerConditionSecret,
	BrokerConditionIngress,
	BrokerConditionAddressable,
)

func (r *Reconciler) ReconcileKind(ctx context.Context, b *eventingv1.Broker) pkgreconciler.Event {
	logging.FromContext(ctx).Infow("Reconciling", zap.Any("Broker", b))

	// 1. RabbitMQ Exchange
	// 2. Ingress Deployment
	// 3. K8s Service that points to the Ingress Deployment
	args, err := r.brokerConfig.GetExchangeArgs(ctx, b)
	if err != nil {
		MarkExchangeFailed(&b.Status, "ExchangeCredentialsUnavailable", "Failed to get arguments for creating exchange: %s", err)
		return err
	}
	return r.reconcileRabbitResources(ctx, b, args)
}

// reconcileDeployment reconciles the K8s Deployment 'd'.
func (r *Reconciler) reconcileDeployment(ctx context.Context, d *v1.Deployment) error {
	current, err := r.deploymentLister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		_, err = r.kubeClientSet.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !equality.Semantic.DeepDerivative(d.Spec.Template, current.Spec.Template) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		_, err = r.kubeClientSet.AppsV1().Deployments(desired.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// reconcileService reconciles the K8s Service 'svc'.
func (r *Reconciler) reconcileService(ctx context.Context, svc *corev1.Service) (*corev1.Endpoints, error) {
	current, err := r.serviceLister.Services(svc.Namespace).Get(svc.Name)
	if apierrs.IsNotFound(err) {
		current, err = r.kubeClientSet.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	// spec.clusterIP is immutable and is set on existing services. If we don't set this to the same value, we will
	// encounter an error while updating.
	svc.Spec.ClusterIP = current.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(svc.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = svc.Spec
		if _, err = r.kubeClientSet.CoreV1().Services(current.Namespace).Update(ctx, desired, metav1.UpdateOptions{}); err != nil {
			return nil, err
		}
	}

	return r.endpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
}

// reconcileIngressDeploymentCRD reconciles the Ingress Deployment.
func (r *Reconciler) reconcileIngressDeployment(ctx context.Context, b *eventingv1.Broker, rabbitmqVhost string) error {
	clusterRef, err := r.brokerConfig.GetRabbitMQClusterRef(ctx, b)
	if err != nil {
		return err
	}
	secretName, err := r.rabbit.GetRabbitMQCASecret(ctx, clusterRef)
	if err != nil {
		return err
	}
	resourceRequirements, err := utils.GetResourceRequirements(b.ObjectMeta)
	if err != nil {
		return err
	}
	expected := resources.MakeIngressDeployment(&resources.IngressArgs{
		Broker:               b,
		Image:                r.ingressImage,
		RabbitMQSecretName:   rabbit.SecretName(b.Name, "broker"),
		RabbitMQCASecretName: secretName,
		BrokerUrlSecretKey:   rabbit.BrokerURLSecretKey,
		RabbitMQVhost:        rabbitmqVhost,
		Configs:              r.configs,
		ResourceRequirements: resourceRequirements,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileIngressService reconciles the Ingress Service.
func (r *Reconciler) reconcileIngressService(ctx context.Context, b *eventingv1.Broker) (*corev1.Endpoints, error) {
	expected := resources.MakeIngressService(b)
	return r.reconcileService(ctx, expected)
}

// reconcileDLXDispatcherDeployment reconciles Brokers DLX dispatcher deployment.
func (r *Reconciler) reconcileDLXDispatcherDeployment(ctx context.Context, b *eventingv1.Broker, sub *duckv1.Addressable, rabbitmqVhost string) error {
	// If there's a sub, then reconcile the deployment as usual.
	if sub != nil {
		clusterRef, err := r.brokerConfig.GetRabbitMQClusterRef(ctx, b)
		if err != nil {
			return err
		}
		secretName, err := r.rabbit.GetRabbitMQCASecret(ctx, clusterRef)
		if err != nil {
			return err
		}
		requirements, err := utils.GetResourceRequirements(b.ObjectMeta)
		if err != nil {
			return err
		}
		expected := resources.MakeDispatcherDeployment(&resources.DispatcherArgs{
			Broker: b,
			Image:  r.dispatcherImage,
			//ServiceAccountName string
			Delivery:             b.Spec.Delivery,
			RabbitMQSecretName:   rabbit.SecretName(b.Name, "broker"),
			RabbitMQCASecretName: secretName,
			RabbitMQVhost:        rabbitmqVhost,
			QueueName:            naming.CreateBrokerDeadLetterQueueName(b),
			BrokerUrlSecretKey:   rabbit.BrokerURLSecretKey,
			Subscriber:           sub,
			BrokerIngressURL:     b.Status.Address.URL,
			Configs:              r.configs,
			ResourceRequirements: requirements,
			DLX:                  true,
		})
		return r.reconcileDeployment(ctx, expected)
	}
	// However if there's not, then ensure that one doesn't exist and delete it if does.
	dispatcherName := resources.DispatcherName(b.Name)
	_, err := r.deploymentLister.Deployments(b.Namespace).Get(dispatcherName)
	if apierrs.IsNotFound(err) {
		// Not there, we're good.
		return nil
	} else if err != nil {
		return err
	}
	// It's there but it shouldn't, so delete.
	return r.kubeClientSet.AppsV1().Deployments(b.Namespace).Delete(ctx, dispatcherName, metav1.DeleteOptions{})
}

func (r *Reconciler) reconcileRabbitResources(ctx context.Context, b *eventingv1.Broker, args *rabbit.ExchangeArgs) error {
	args.Name = naming.BrokerExchangeName(b, false)
	exchange, err := r.rabbit.ReconcileExchange(ctx, args)
	if err != nil {
		MarkExchangeFailed(&b.Status, "ExchangeFailure", fmt.Sprintf("Failed to reconcile exchange %q: %s", naming.BrokerExchangeName(args.Broker, false), err))
		return err
	}
	if !exchange.Ready {
		logging.FromContext(ctx).Warnf("Exchange %q is not ready", exchange.Name)
		MarkExchangeFailed(&b.Status, "ExchangeFailure", fmt.Sprintf("exchange %q is not ready", exchange.Name))
		return nil
	}
	args.Name = naming.BrokerExchangeName(b, true)
	MarkExchangeReady(&b.Status)

	if deadLetterEnabled(b) {
		if err, ready := r.reconcileDeadLetterResources(ctx, b, args); err != nil {
			return err
		} else if !ready {
			return nil
		}
	} else {
		MarkDLXNotConfigured(&b.Status)
		MarkDeadLetterSinkNotConfigured(&b.Status)
	}

	return r.reconcileCommonIngressResources(
		ctx,
		rabbit.MakeSecret(args.Broker.Name, "broker", args.Namespace, args.RabbitMQURL.String(), args.Broker),
		b,
		args.RabbitMQVhost,
	)
}

func (r *Reconciler) reconcileDeadLetterResources(ctx context.Context, b *eventingv1.Broker, args *rabbit.ExchangeArgs) (error, bool) {
	dlxExchange, err := r.rabbit.ReconcileExchange(ctx, args)
	if err != nil {
		MarkExchangeFailed(&b.Status, "ExchangeFailure", fmt.Sprintf("Failed to reconcile DLX exchange %q: %s", naming.BrokerExchangeName(args.Broker, true), err))
		return err, false
	}
	if !dlxExchange.Ready {
		logging.FromContext(ctx).Warnf("DLX exchange %q is not ready", dlxExchange.Name)
		MarkExchangeFailed(&b.Status, "ExchangeFailure", fmt.Sprintf("DLX exchange %q is not ready", dlxExchange.Name))
		return nil, false
	}

	clusterRef, err := r.brokerConfig.GetRabbitMQClusterRef(ctx, b)
	if err != nil {
		return err, false
	}
	queueType, err := r.brokerConfig.GetQueueType(ctx, b)
	if err != nil {
		return err, false
	}
	queue, err := r.rabbit.ReconcileQueue(ctx, &rabbit.QueueArgs{
		Name:                     naming.CreateBrokerDeadLetterQueueName(b),
		Namespace:                b.Namespace,
		RabbitMQVhost:            args.RabbitMQVhost,
		RabbitmqClusterReference: clusterRef,
		Owner:                    *kmeta.NewControllerRef(b),
		Labels:                   rabbit.Labels(b, nil, nil),
		QueueType:                queueType,
	})
	if err != nil {
		MarkDLXFailed(&b.Status, "QueueFailure", fmt.Sprintf("Failed to reconcile Dead Letter Queue %q : %s", naming.CreateBrokerDeadLetterQueueName(b), err))
		return err, false
	}
	if !queue.Ready {
		logging.FromContext(ctx).Warnf("Queue %q is not ready", queue.Name)
		MarkDLXFailed(&b.Status, "QueueFailure", fmt.Sprintf("Dead Letter Queue %q is not ready", queue.Name))
		return nil, false
	}
	MarkDLXReady(&b.Status)

	bindingName := naming.CreateBrokerDeadLetterQueueName(b)
	binding, err := r.rabbit.ReconcileBinding(ctx, &rabbit.BindingArgs{
		Name:                     bindingName,
		Namespace:                b.Namespace,
		RabbitmqClusterReference: clusterRef,
		RabbitMQVhost:            args.RabbitMQVhost,
		Source:                   naming.BrokerExchangeName(b, true),
		Destination:              bindingName,
		Owner:                    *kmeta.NewControllerRef(b),
		Labels:                   rabbit.Labels(b, nil, nil),
		Filters: map[string]string{
			rabbit.DLQBindingKey: b.Name,
		},
	})
	if err != nil {
		// NB, binding has the same name as the queue.
		MarkDeadLetterSinkFailed(&b.Status, "DLQ binding", fmt.Sprintf("Failed to reconcile DLQ binding %q : %s", bindingName, err))
		return err, false
	}
	if !binding.Ready {
		logging.FromContext(ctx).Warnf("Binding %q is not ready", binding.Name)
		MarkDeadLetterSinkFailed(&b.Status, "DLQ binding", fmt.Sprintf("DLQ binding %q is not ready", binding.Name))
		return nil, false
	}

	policyName := naming.CreateBrokerDeadLetterQueueName(b) // same name as the DL queue
	policy, err := r.rabbit.ReconcileBrokerDLXPolicy(ctx, &rabbit.QueueArgs{
		Name:                     policyName,
		Namespace:                b.Namespace,
		RabbitMQVhost:            args.RabbitMQVhost,
		RabbitmqClusterReference: clusterRef,
		Owner:                    *kmeta.NewControllerRef(b),
		Labels:                   rabbit.Labels(b, nil, nil),
		DLXName:                  pointer.String(args.Name),
		BrokerUID:                string(b.GetUID()),
	})
	if err != nil {
		MarkDeadLetterSinkFailed(&b.Status, "PolicyFailure", fmt.Sprintf("Failed to reconcile RabbitMQ Policy %q : %s", policyName, err))
		return err, false
	}
	if !policy.Ready {
		logging.FromContext(ctx).Warnf("RabbitMQ Policy %q is not ready", policyName)
		MarkDeadLetterSinkFailed(&b.Status, "PolicyFailure", fmt.Sprintf("RabbitMQ Policy %q is not ready", policyName))
		return nil, false
	}

	MarkDeadLetterSinkReady(&b.Status)
	return nil, true
}

// reconcileCommonIngressResources that are shared between implementations using CRDs or libraries.
func (r *Reconciler) reconcileCommonIngressResources(ctx context.Context, s *corev1.Secret, b *eventingv1.Broker, rabbitmqVhost string) error {
	if err := rabbit.ReconcileSecret(ctx, r.secretLister, r.kubeClientSet, s); err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling Secret", zap.Error(err))
		MarkSecretFailed(&b.Status, "SecretFailure", "Failed to reconcile secret: %s", err)
		return err
	}
	MarkSecretReady(&b.Status)

	if err := r.reconcileIngressDeployment(ctx, b, rabbitmqVhost); err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling ingress Deployment", zap.Error(err))
		MarkIngressFailed(&b.Status, "DeploymentFailure", "Failed to reconcile deployment: %s", err)
		return err
	}

	ingressEndpoints, err := r.reconcileIngressService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling ingress Service", zap.Error(err))
		MarkIngressFailed(&b.Status, "ServiceFailure", "Failed to reconcile service: %s", err)
		return err
	}
	PropagateIngressAvailability(&b.Status, ingressEndpoints)

	b.Status.SetAddress(&duckv1.Addressable{
		Name: pointer.String("http"),
		URL: &apis.URL{
			Scheme: "http",
			Host:   network.GetServiceHostname(ingressEndpoints.GetName(), ingressEndpoints.GetNamespace()),
		},
	})

	// If there's a Dead Letter Sink, then create a dispatcher for it. Note that this is for
	// the whole broker, unlike for the Trigger, where we create one dispatcher per Trigger.
	var dlsAddressable *duckv1.Addressable
	if deadLetterEnabled(b) {
		dlsAddressable, err = r.uriResolver.AddressableFromDestinationV1(ctx, *b.Spec.Delivery.DeadLetterSink, b)
		if err != nil {
			logging.FromContext(ctx).Error("Unable to get the DeadLetterSink Addressable", zap.Error(err))
			b.Status.MarkDeadLetterSinkResolvedFailed("Unable to get the DeadLetterSink's Addressable", "%v", err)
			return err
		}
		b.Status.MarkDeadLetterSinkResolvedSucceeded(eventingduckv1.NewDeliveryStatusFromAddressable(dlsAddressable))
	} else {
		b.Status.MarkDeadLetterSinkNotConfigured()
	}

	// Note that if we didn't actually resolve the URI above, as in it's left as nil it's ok to pass here
	// it deals with it properly.
	if err := r.reconcileDLXDispatcherDeployment(ctx, b, dlsAddressable, rabbitmqVhost); err != nil {
		logging.FromContext(ctx).Error("Problem reconciling DLX dispatcher Deployment", zap.Error(err))
		MarkDeadLetterSinkFailed(&b.Status, "DeploymentFailure", "%v", err)
		return err
	}
	return nil
}

func deadLetterEnabled(b *eventingv1.Broker) bool {
	return b.Spec.Delivery != nil && b.Spec.Delivery.DeadLetterSink != nil
}
