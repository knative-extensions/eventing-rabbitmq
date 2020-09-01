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
	"errors"
	"fmt"

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	BrokerUrlSecretKey = "brokerURL"

	// Name of the corev1.Events emitted from the Broker reconciliation process.
	brokerReconciled = "BrokerReconciled"
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

var brokerGVK = v1beta1.SchemeGroupVersion.WithKind("Broker")

// ReconcilerArgs are the arguments needed to create a broker.Reconciler.
type ReconcilerArgs struct {
	IngressImage              string
	IngressServiceAccountName string
}

func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, brokerReconciled, "Broker reconciled: \"%s/%s\"", namespace, name)
}

func (r *Reconciler) ReconcileKind(ctx context.Context, b *v1beta1.Broker) pkgreconciler.Event {
	logging.FromContext(ctx).Debug("Reconciling", zap.Any("Broker", b))
	//b.Status.InitializeConditions()

	// TODO broker coupled to channels
	b.Status.PropagateTriggerChannelReadiness(&duckv1beta1.ChannelableStatus{
		AddressStatus: pkgduckv1.AddressStatus{
			Address: &pkgduckv1.Addressable{},
		},
	})
	b.Status.ObservedGeneration = b.Generation

	// 1. RabbitMQ Exchange
	// 2. Ingress Deployment
	// 3. K8s Service that points to the Ingress Deployment
	// 4. KEDA TriggerAuthentication to be used by ScaledObjects

	rabbitmqURL, err := r.rabbitmqURL(ctx, b)
	if err != nil {
		return err
	}

	err = resources.DeclareExchange(&resources.ExchangeArgs{
		Broker:      b,
		RabbitmqURL: rabbitmqURL,
	})
	if err != nil {
		logging.FromContext(ctx).Error("Problem creating RabbitMQ Exchange", zap.Error(err))
		b.Status.MarkIngressFailed("ExchangeFailure", "%v", err)
		return err
	}

	if err := r.reconcileIngressDeployment(ctx, b); err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Deployment", zap.Error(err))
		b.Status.MarkIngressFailed("DeploymentFailure", "%v", err)
		return err
	}

	ingressEndpoints, err := r.reconcileIngressService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Service", zap.Error(err))
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
		logging.FromContext(ctx).Error("Problem creating TriggerAuthentication", zap.Error(err))
		b.Status.MarkIngressFailed("TriggerAuthenticationFailure", "%v", err)
		return err
	}

	// So, at this point the Broker is ready and everything should be solid
	// for the triggers to act upon.
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, b *v1beta1.Broker) pkgreconciler.Event {
	rabbitmqURL, err := r.rabbitmqURL(ctx, b)
	if err != nil {
		return err
	}
	resources.DeleteExchange(&resources.ExchangeArgs{
		Broker:      b,
		RabbitmqURL: rabbitmqURL,
	})
	return newReconciledNormal(b.Namespace, b.Name)
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
		current, err = r.kubeClientSet.CoreV1().Services(current.Namespace).Update(desired)
		if err != nil {
			return nil, err
		}
	}

	return r.endpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
}

// reconcileIngressDeploymentCRD reconciles the Ingress Deployment.
func (r *Reconciler) reconcileIngressDeployment(ctx context.Context, b *v1beta1.Broker) error {
	secret, err := r.getRabbitmqSecret(ctx, b)
	if err != nil {
		return err
	}
	expected := resources.MakeIngressDeployment(&resources.IngressArgs{
		Broker:             b,
		Image:              r.ingressImage,
		RabbitMQSecretName: secret.Name,
		BrokerUrlSecretKey: BrokerUrlSecretKey,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileIngressService reconciles the Ingress Service.
func (r *Reconciler) reconcileIngressService(ctx context.Context, b *v1beta1.Broker) (*corev1.Endpoints, error) {
	expected := resources.MakeIngressService(b)
	return r.reconcileService(ctx, expected)
}

/* TODO: Enable once we start filtering by classes of brokers
func brokerLabels(name string) map[string]string {
	return map[string]string{
		brokerAnnotationKey: name,
	}
}
*/

func (r *Reconciler) reconcileScaleTriggerAuthentication(ctx context.Context, b *v1beta1.Broker) (*kedav1alpha1.TriggerAuthentication, error) {
	secret, err := r.getRabbitmqSecret(ctx, b)
	if err != nil {
		return nil, err
	}
	namespace := b.Namespace
	triggerAuthentication := resources.MakeTriggerAuthentication(&resources.TriggerAuthenticationArgs{
		Broker:     b,
		SecretName: secret.Name,
		SecretKey:  BrokerUrlSecretKey,
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

func (r *Reconciler) getRabbitmqSecret(ctx context.Context, b *v1beta1.Broker) (*corev1.Secret, error) {
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

func (r *Reconciler) rabbitmqURL(ctx context.Context, b *v1beta1.Broker) (string, error) {
	s, err := r.getRabbitmqSecret(ctx, b)
	if err != nil {
		return "", err
	}
	val := s.Data[BrokerUrlSecretKey]
	if val == nil {
		return "", fmt.Errorf("Secret missing key %s", BrokerUrlSecretKey)
	}
	return string(val), nil
}
