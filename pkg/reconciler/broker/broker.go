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

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"

	"knative.dev/eventing-rabbitmq/pkg/reconciler/broker/resources"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/broker"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"

	kedaclientset "knative.dev/eventing-rabbitmq/pkg/internal/thirdparty/keda/client/clientset/versioned"
	kedalisters "knative.dev/eventing-rabbitmq/pkg/internal/thirdparty/keda/client/listers/keda/v1alpha1"
	kedav1alpha1 "knative.dev/eventing-rabbitmq/pkg/internal/thirdparty/keda/v1alpha1"

	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/names"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

type Reconciler struct {
	kedaClientset     kedaclientset.Interface
	eventingClientSet clientset.Interface
	dynamicClientSet  dynamic.Interface
	kubeClientSet     kubernetes.Interface

	// listers index properties about resources
	brokerLister                eventinglisters.BrokerLister
	serviceLister               corev1listers.ServiceLister
	endpointsLister             corev1listers.EndpointsLister
	secretLister                corev1listers.SecretLister
	deploymentLister            appsv1listers.DeploymentLister
	triggerAuthenticationLister kedalisters.TriggerAuthenticationLister

	ingressImage              string
	ingressServiceAccountName string

	// Dynamic tracker to track KResources. In particular, it tracks the dependency between Triggers and Sources.
	kresourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. In particular, it tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver

	// If specified, only reconcile brokers with these labels
	brokerClass string
}

// Check that our Reconciler implements Interface
var _ brokerreconciler.Interface = (*Reconciler)(nil)
var _ brokerreconciler.Finalizer = (*Reconciler)(nil)

// ReconcilerArgs are the arguments needed to create a broker.Reconciler.
type ReconcilerArgs struct {
	IngressImage              string
	IngressServiceAccountName string
}

func (r *Reconciler) ReconcileKind(ctx context.Context, b *v1beta1.Broker) pkgreconciler.Event {
	logging.FromContext(ctx).Debugw("Reconciling", zap.Any("Broker", b))

	// TODO: broker coupled to channels
	b.Status.PropagateTriggerChannelReadiness(&duckv1beta1.ChannelableStatus{
		AddressStatus: pkgduckv1.AddressStatus{
			Address: &pkgduckv1.Addressable{},
		},
	})

	// 1. RabbitMQ Exchange
	// 2. Ingress Deployment
	// 3. K8s Service that points to the Ingress Deployment
	// 4. KEDA TriggerAuthentication to be used by ScaledObjects

	args, err := r.getExchangeArgs(ctx, b)
	if err != nil {
		return err
	}

	s, err := resources.DeclareExchange(args)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem creating RabbitMQ Exchange", zap.Error(err))
		b.Status.MarkIngressFailed("ExchangeFailure", "%v", err)
		return err
	}

	if err := r.reconcileSecret(ctx, s); err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling Secret", zap.Error(err))
		b.Status.MarkIngressFailed("SecretFailure", "%v", err)
		return err
	}

	if err := r.reconcileIngressDeployment(ctx, b); err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling ingress Deployment", zap.Error(err))
		b.Status.MarkIngressFailed("DeploymentFailure", "%v", err)
		return err
	}

	ingressEndpoints, err := r.reconcileIngressService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling ingress Service", zap.Error(err))
		b.Status.MarkIngressFailed("ServiceFailure", "%v", err)
		return err
	}
	b.Status.PropagateIngressAvailability(ingressEndpoints)
	// TODO something else, faking this for the broker status
	b.Status.PropagateFilterAvailability(ingressEndpoints)

	b.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(ingressEndpoints.GetName(), ingressEndpoints.GetNamespace()),
	})

	_, err = r.reconcileScaleTriggerAuthentication(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem creating TriggerAuthentication", zap.Error(err))
		b.Status.MarkIngressFailed("TriggerAuthenticationFailure", "%v", err)
		return err
	}

	// So, at this point the Broker is ready and everything should be solid
	// for the triggers to act upon.
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, b *v1beta1.Broker) pkgreconciler.Event {
	args, err := r.getExchangeArgs(ctx, b)
	if err != nil {
		return err
	}
	if err := resources.DeleteExchange(args); err != nil {
		logging.FromContext(ctx).Errorw("Problem deleting exchange", zap.Error(err))
	}
	return nil
}

// reconcileSecret reconciles the K8s Secret 's'.
func (r *Reconciler) reconcileSecret(ctx context.Context, s *corev1.Secret) error {
	current, err := r.secretLister.Secrets(s.Namespace).Get(s.Name)
	if apierrs.IsNotFound(err) {
		_, err = r.kubeClientSet.CoreV1().Secrets(s.Namespace).Create(s)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !equality.Semantic.DeepDerivative(s.StringData, current.StringData) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.StringData = s.StringData
		_, err = r.kubeClientSet.CoreV1().Secrets(desired.Namespace).Update(desired)
		if err != nil {
			return err
		}
	}
	return nil
}

// reconcileDeployment reconciles the K8s Deployment 'd'.
func (r *Reconciler) reconcileDeployment(ctx context.Context, d *v1.Deployment) error {
	current, err := r.deploymentLister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		_, err = r.kubeClientSet.AppsV1().Deployments(d.Namespace).Create(d)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !equality.Semantic.DeepDerivative(d.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		_, err = r.kubeClientSet.AppsV1().Deployments(desired.Namespace).Update(desired)
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
		current, err = r.kubeClientSet.CoreV1().Services(svc.Namespace).Create(svc)
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
		if _, err := r.kubeClientSet.CoreV1().Services(current.Namespace).Update(desired); err != nil {
			return nil, err
		}
	}

	return r.endpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
}

// reconcileIngressDeploymentCRD reconciles the Ingress Deployment.
func (r *Reconciler) reconcileIngressDeployment(ctx context.Context, b *v1beta1.Broker) error {
	expected := resources.MakeIngressDeployment(&resources.IngressArgs{
		Broker:             b,
		Image:              r.ingressImage,
		RabbitMQSecretName: resources.SecretName(b.Name),
		BrokerUrlSecretKey: resources.BrokerURLSecretKey,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileIngressService reconciles the Ingress Service.
func (r *Reconciler) reconcileIngressService(ctx context.Context, b *v1beta1.Broker) (*corev1.Endpoints, error) {
	expected := resources.MakeIngressService(b)
	return r.reconcileService(ctx, expected)
}

func (r *Reconciler) reconcileScaleTriggerAuthentication(ctx context.Context, b *v1beta1.Broker) (*kedav1alpha1.TriggerAuthentication, error) {
	namespace := b.Namespace
	triggerAuthentication := resources.MakeTriggerAuthentication(&resources.TriggerAuthenticationArgs{
		Broker:     b,
		SecretName: resources.SecretName(b.Name),
		SecretKey:  resources.BrokerURLSecretKey,
	})

	current, err := r.triggerAuthenticationLister.TriggerAuthentications(namespace).Get(triggerAuthentication.Name)
	if apierrs.IsNotFound(err) {
		_, err = r.kedaClientset.KedaV1alpha1().TriggerAuthentications(namespace).Create(triggerAuthentication)
		if err != nil {
			return nil, err
		}
		return triggerAuthentication, nil
	}
	if err != nil {
		return nil, err
	}
	if !equality.Semantic.DeepDerivative(triggerAuthentication.Spec, triggerAuthentication.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = triggerAuthentication.Spec
		_, err = r.kedaClientset.KedaV1alpha1().TriggerAuthentications(namespace).Update(desired)
		if err != nil {
			return nil, err
		}
		return desired, nil
	}
	return current, nil
}
