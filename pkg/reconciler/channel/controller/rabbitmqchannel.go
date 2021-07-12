/*
Copyright 2019 The Knative Authors

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

package controller

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"
	"knative.dev/pkg/reconciler"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	rabbitclientset "github.com/rabbitmq/messaging-topology-operator/pkg/generated/clientset/versioned"
	rabbitlisters "github.com/rabbitmq/messaging-topology-operator/pkg/generated/listers/rabbitmq.com/v1beta1"

	messagingv1beta1 "knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	rabbitmqChannelReconciler "knative.dev/eventing-rabbitmq/pkg/client/injection/reconciler/messaging/v1beta1/rabbitmqchannel"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/channel/controller/resources"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	apisduck "knative.dev/pkg/apis/duck"
)

const (
	ReconcilerName = "RabbitmqChannel"

	// Name of the corev1.Events emitted from the reconciliation process.
	dispatcherDeploymentNotFound = "DispatcherDeploymentDoesNotExist"
	dispatcherDeploymentFailed   = "DispatcherDeploymentFailed"
	dispatcherServiceFailed      = "DispatcherServiceFailed"
	dispatcherServiceNotFound    = "DispatcherServiceDoesNotExist"
	dispatcherEndpointsNotFound  = "DispatcherEndpointsDoesNotExist"
	dispatcherEndpointsFailed    = "DispatcherEndpointsFailed"
	channelServiceFailed         = "ChannelServiceFailed"

	dispatcherName = "rabbitmq-ch-dispatcher"
)

// Reconciler reconciles Rabbitmq Channels.
type Reconciler struct {
	kubeClientSet    kubernetes.Interface
	rabbitClientSet  rabbitclientset.Interface
	dynamicClientSet dynamic.Interface

	dispatcherNamespace      string
	dispatcherDeploymentName string
	dispatcherServiceName    string

	deploymentLister appsv1listers.DeploymentLister
	serviceLister    corev1listers.ServiceLister
	endpointsLister  corev1listers.EndpointsLister
	rabbitLister     apisduck.InformerFactory
	exchangeLister   rabbitlisters.ExchangeLister
	queueLister      rabbitlisters.QueueLister
	bindingLister    rabbitlisters.BindingLister

	ingressImage    string
	dispatcherImage string
}

var _ rabbitmqChannelReconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, channel *messagingv1beta1.RabbitmqChannel) reconciler.Event {
	logger := logging.FromContext(ctx)

	// Reconcile RabbitmqChannel involves:
	// * Creating an Exchange (Fanout)
	// * Creating a DLX
	// * Queue for consuming events
	// * Receiver for getting events into the Exchange. This is pretty much identical to Broker ingress.
	args, err := r.getExchangeArgs(ctx, channel)
	if err != nil {
		MarkExchangeFailed(&b.Status, "ExchangeCredentialsUnavailable", "Failed to get arguments for creating exchange: %s", err)
		return err
	}

	exchangeArgs := resources.ExchangeArgs{
		Channel: channel,
	}
	exchange, err := r.reconcileExchange(ctx)
	// Then for each Subscription we need to create a Dispatcher. This is again pretty much identical to Broker Dispatcher
	// but there's no filtering necessary.

	// This means creating a Binding for it as well as a Dispatcher deployment.
	// Get the Dispatcher Deployment and propagate the status to the Channel
	if d, err := r.deploymentLister.Deployments(r.dispatcherNamespace).Get(r.dispatcherDeploymentName); err != nil {
		logger.Error("Unable to get the dispatcher Deployment", zap.Error(err))
		if apierrs.IsNotFound(err) {
			channel.Status.MarkDispatcherFailed(dispatcherDeploymentNotFound, "Dispatcher Deployment does not exist")
		} else {
			channel.Status.MarkDispatcherFailed(dispatcherDeploymentFailed, "Failed to get dispatcher Deployment")
		}
	} else {
		channel.Status.PropagateDispatcherStatus(&d.Status)
	}

	// Get the Dispatcher Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	if _, err := r.serviceLister.Services(r.dispatcherNamespace).Get(r.dispatcherServiceName); err != nil {
		logger.Error("Unable to get the dispatcher service", zap.Error(err))
		if apierrs.IsNotFound(err) {
			channel.Status.MarkServiceFailed(dispatcherServiceNotFound, "Dispatcher Service does not exist")
		} else {
			channel.Status.MarkServiceFailed(dispatcherServiceFailed, "Failed to get dispatcher service")
		}
	} else {
		channel.Status.MarkServiceTrue()
	}

	// Get the Dispatcher Service Endpoints and propagate the status to the Channel
	// endpoints has the same name as the service, so not a bug.
	if e, err := r.endpointsLister.Endpoints(r.dispatcherNamespace).Get(r.dispatcherServiceName); err != nil {
		logger.Error("Unable to get the dispatcher endpoints", zap.Error(err))
		if apierrs.IsNotFound(err) {
			channel.Status.MarkEndpointsFailed(dispatcherEndpointsNotFound, "Dispatcher Endpoints does not exist")
		} else {
			channel.Status.MarkEndpointsFailed(dispatcherEndpointsFailed, "Failed to get dispatcher endpoints")
		}
	} else {
		if len(e.Subsets) == 0 {
			channel.Status.MarkEndpointsFailed("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service")
		} else {
			channel.Status.MarkEndpointsTrue()
		}
	}

	// Reconcile the k8s service representing the actual Channel. It points to the Dispatcher service via ExternalName
	if svc, err := r.reconcileChannelService(ctx, channel); err != nil {
		channel.Status.MarkChannelServiceFailed(channelServiceFailed, fmt.Sprintf("Channel Service failed: %s", err))
	} else {
		channel.Status.MarkChannelServiceTrue()
		channel.Status.SetAddress(&apis.URL{
			Scheme: "http",
			Host:   network.GetServiceHostname(svc.Name, svc.Namespace),
		})
	}

	// Ok, so now the Dispatcher Deployment & Service have been created, we're golden since the
	// dispatcher watches the Channel and where it needs to dispatch events to.
	return nil
}

func (r *Reconciler) reconcileChannelService(ctx context.Context, channel *messagingv1beta1.RabbitmqChannel) (*corev1.Service, error) {
	logger := logging.FromContext(ctx)
	// Get the  Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	svc, err := r.serviceLister.Services(channel.Namespace).Get(resources.MakeChannelServiceName(channel.Name))
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = resources.MakeK8sService(channel, resources.ExternalService(r.dispatcherNamespace, r.dispatcherServiceName))
			if err != nil {
				logger.Error("Failed to create the channel service object", zap.Error(err))
				return nil, err
			}
			svc, err = r.kubeClientSet.CoreV1().Services(channel.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			if err != nil {
				logger.Error("Failed to create the channel service", zap.Error(err))
				return nil, err
			}
			return svc, nil
		}
		logger.Error("Unable to get the channel service", zap.Error(err))
		return nil, err
	}
	// Check to make sure that the NatssChannel owns this service and if not, complain.
	if !metav1.IsControlledBy(svc, channel) {
		return nil, fmt.Errorf("natsschannel: %s/%s does not own Service: %q", channel.Namespace, channel.Name, svc.Name)
	}
	return svc, nil
}

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
func (r *Reconciler) reconcileIngressDeployment(ctx context.Context, c *messagingv1beta1.RabbitmqChannel) error {
	expected := resources.MakeIngressDeployment(&resources.IngressArgs{
		Channel:            c,
		Image:              r.ingressImage,
		RabbitMQSecretName: resources.SecretName(c.Name),
		BrokerUrlSecretKey: resources.BrokerURLSecretKey,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileIngressService reconciles the Ingress Service.
func (r *Reconciler) reconcileIngressService(ctx context.Context, c *messagingv1beta1.RabbitmqChannel) (*corev1.Endpoints, error) {
	expected := resources.MakeIngressService(c)
	return r.reconcileService(ctx, expected)
}

// reconcileDLXDispatcherDeployment reconciles Channels DLX dispatcher deployment.
func (r *Reconciler) reconcileDLXDispatcherDeployment(ctx context.Context, c *messagingv1beta1.RabbitmqChannel, sub *apis.URL) error {
	// If there's a sub, then reconcile the deployment as usual.
	if sub != nil {
		expected := resources.MakeDispatcherDeployment(&resources.DispatcherArgs{
			Channel:            c,
			Image:              r.dispatcherImage,
			RabbitMQSecretName: resources.SecretName(c.Name),
			QueueName:          naming.CreateChannelDeadLetterQueueName(c),
			BrokerUrlSecretKey: resources.BrokerURLSecretKey,
			Subscriber:         sub,
		})
		return r.reconcileDeployment(ctx, expected)
	}
	// However if there's not, then ensure that one doesn't exist and delete it if does.
	dispatcherName := resources.DispatcherName(c.Name)
	_, err := r.deploymentLister.Deployments(c.Namespace).Get(dispatcherName)
	if apierrs.IsNotFound(err) {
		// Not there, we're good.
		return nil
	} else if err != nil {
		return err
	}
	// It's there but it shouldn't, so delete.
	return r.kubeClientSet.AppsV1().Deployments(c.Namespace).Delete(ctx, dispatcherName, metav1.DeleteOptions{})
}

func (r *Reconciler) reconcileExchange(ctx context.Context, args *resources.ExchangeArgs) (*v1beta1.Exchange, error) {
	want := resources.NewExchange(ctx, args)
	current, err := r.exchangeLister.Exchanges(args.Channel.Namespace).Get(naming.ChannelExchangeName(args.Channel, args.DLX))
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq exchange", zap.String("exchange name", want.Name))
		return r.rabbitClientSet.RabbitmqV1beta1().Exchanges(args.Channel.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		return r.rabbitClientSet.RabbitmqV1beta1().Exchanges(args.Channel.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	return current, nil
}

// reconcileChannelQueue reconciles the Channels DLQ
func (r *Reconciler) reconcileChannelQueue(ctx context.Context, c *messagingv1beta1.RabbitmqChannel) (*v1beta1.Queue, error) {
	queueName := naming.CreateChannelDeadLetterQueueName(c)
	want := resources.NewQueue(ctx, c, nil)
	current, err := r.queueLister.Queues(c.Namespace).Get(queueName)
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq queue for channel", zap.String("queue name", want.Name))
		return r.rabbitClientSet.RabbitmqV1beta1().Queues(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		return r.rabbitClientSet.RabbitmqV1beta1().Queues(c.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	return current, nil
}

func (r *Reconciler) reconcileSubscriberQueue(ctx context.Context, c *messagingv1beta1.RabbitmqChannel, s *eventingduckv1.SubscriberSpec) (*v1beta1.Queue, error) {
	queueName := naming.CreateSubscriberQueueName(c, s)
	want := resources.NewQueue(ctx, c, s)
	current, err := r.queueLister.Queues(c.Namespace).Get(queueName)
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating rabbitmq queue for subscription", zap.String("queue name", want.Name))
		return r.rabbitClientSet.RabbitmqV1beta1().Queues(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		return r.rabbitClientSet.RabbitmqV1beta1().Queues(c.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	return current, nil
}

func (r *Reconciler) reconcileBinding(ctx context.Context, c *messagingv1beta1.RabbitmqChannel) (*v1beta1.Binding, error) {
	// We can use the same name for queue / binding to keep things simpler
	bindingName := naming.CreateChannelDeadLetterQueueName(c)

	want, err := resources.NewBinding(ctx, c, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create the binding spec: %w", err)
	}
	current, err := r.bindingLister.Bindings(c.Namespace).Get(bindingName)
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Infow("Creating rabbitmq binding", zap.String("binding name", want.Name))
		return r.rabbitClientSet.RabbitmqV1beta1().Bindings(c.Namespace).Create(ctx, want, metav1.CreateOptions{})
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(want.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = want.Spec
		return r.rabbitClientSet.RabbitmqV1beta1().Bindings(c.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	}
	return current, nil
}
