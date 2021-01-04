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

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	rabbitmqChannelReconciler "knative.dev/eventing-rabbitmq/pkg/client/injection/reconciler/messaging/v1beta1/rabbitmqchannel"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/channel/controller/resources"
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

// Reconciler reconciles NATSS Channels.
type Reconciler struct {
	kubeClientSet kubernetes.Interface

	dispatcherNamespace      string
	dispatcherDeploymentName string
	dispatcherServiceName    string

	deploymentLister appsv1listers.DeploymentLister
	serviceLister    corev1listers.ServiceLister
	endpointsLister  corev1listers.EndpointsLister
}

var _ rabbitmqChannelReconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, channel *v1beta1.RabbitmqChannel) reconciler.Event {
	logger := logging.FromContext(ctx)

	// We reconcile the status of the Channel by looking at:
	// 1. Dispatcher Deployment for it's readiness.
	// 2. Dispatcher k8s Service for it's existence.
	// 3. Dispatcher endpoints to ensure that there's something backing the Service.
	// 4. K8s service representing the channel that will use ExternalName to point to the Dispatcher k8s service.

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

func (r *Reconciler) reconcileChannelService(ctx context.Context, channel *v1beta1.RabbitmqChannel) (*corev1.Service, error) {
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
