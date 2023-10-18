/*
Copyright 2022 The Knative Authors

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

package rabbitmq

import (
	"context"
	"fmt"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	naming "knative.dev/eventing-rabbitmq/pkg/rabbitmqnaming"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	rabbitlisters "knative.dev/eventing-rabbitmq/third_party/pkg/client/listers/rabbitmq.com/v1beta1"
	eventingutils "knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-rabbitmq/pkg/client/clientset/versioned"
	reconcilerrabbitmqsource "knative.dev/eventing-rabbitmq/pkg/client/injection/reconciler/sources/v1alpha1/rabbitmqsource"
	listers "knative.dev/eventing-rabbitmq/pkg/client/listers/sources/v1alpha1"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/source/resources"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	pkgLogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

const (
	raImageEnvVar                   = "RABBITMQ_RA_IMAGE"
	rabbitmqSourceDeploymentCreated = "RabbitmqSourceDeploymentCreated"
	rabbitmqSourceDeploymentUpdated = "RabbitmqSourceDeploymentUpdated"
	rabbitmqSourceDeploymentDeleted = "RabbitmqSourceDeploymentDeleted"
	rabbitmqSourceDeploymentFailed  = "RabbitmqSourceDeploymentUpdated"
	component                       = "rabbitmqsource"
)

func newDeploymentCreated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, rabbitmqSourceDeploymentCreated, "RabbitmqSource created deployment: \"%s/%s\"", namespace, name)
}

func deploymentUpdated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, rabbitmqSourceDeploymentUpdated, "RabbitmqSource updated deployment: \"%s/%s\"", namespace, name)
}

func newDeploymentFailed(namespace, name string, err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, rabbitmqSourceDeploymentFailed, "RabbitmqSource failed to create deployment: \"%s/%s\", %w", namespace, name, err)
}

type Reconciler struct {
	// KubeClientSet allows us to talk to the k8s for core APIs
	KubeClientSet kubernetes.Interface

	receiveAdapterImage string

	rabbitmqLister   listers.RabbitmqSourceLister
	secretLister     corev1listers.SecretLister
	deploymentLister appsv1listers.DeploymentLister
	bindingListener  rabbitlisters.BindingLister
	exchangeLister   rabbitlisters.ExchangeLister
	queueLister      rabbitlisters.QueueLister

	rabbitmqClientSet versioned.Interface
	loggingContext    context.Context
	loggingConfig     *pkgLogging.Config
	metricsConfig     *metrics.ExporterOptions

	sinkResolver *resolver.URIResolver

	rabbit rabbit.Service
}

var _ reconcilerrabbitmqsource.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, src *v1alpha1.RabbitmqSource) pkgreconciler.Event {
	src.Status.InitializeConditions()

	if src.Spec.Sink == nil {
		src.Status.MarkNoSink("SinkMissing", "")
		return fmt.Errorf("spec.sink missing")
	}

	dest := src.Spec.Sink.DeepCopy()
	if dest.Ref != nil {
		if dest.Ref.Namespace == "" {
			dest.Ref.Namespace = src.GetNamespace()
		}
	}
	sinkURI, err := r.sinkResolver.URIFromDestinationV1(ctx, *dest, src)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		return fmt.Errorf("getting sink URI: %v", err)
	}
	src.Status.MarkSink(sinkURI)

	if err := r.reconcileRabbitObjects(ctx, src); err != nil {
		logging.FromContext(ctx).Error("Unable to create RabbitMQ resources", zap.Error(err))
		return err
	}
	src.Status.MarkExchangeReady()

	ra, err := r.createReceiveAdapter(ctx, src, sinkURI)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	src.Status.MarkDeployed(ra)
	src.Status.CloudEventAttributes = r.createCloudEventAttributes(src)
	return nil
}

func (r *Reconciler) reconcileRabbitObjects(ctx context.Context, src *v1alpha1.RabbitmqSource) error {
	logger := logging.FromContext(ctx)
	if src.Spec.RabbitmqClusterReference == nil {
		src.Status.MarkExchangeFailed("RabbitMQClusterReferenceNil", "Failed to get RabbitMQ Cluster reference got %s", src.Spec.RabbitmqClusterReference)
		return fmt.Errorf("rabbitmqSource.Spec.RabbitMQClusterReference is empty")
	}

	rabbitmqURL, err := r.rabbit.RabbitMQURL(ctx, src.Spec.RabbitmqClusterReference)
	if err != nil {
		return err
	}

	if err := rabbit.ReconcileSecret(ctx, r.secretLister, r.KubeClientSet, rabbit.MakeSecret(src.Name, "source", src.Namespace, rabbitmqURL, src)); err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling Secret", zap.Error(err))
		src.Status.MarkSecretFailed("SecretFailure", "Failed to reconcile secret: %s", err)
		return err
	}
	src.Status.MarkSecretReady()

	if src.Spec.RabbitmqResourcesConfig.Predeclared {
		logger.Info("predeclared set to true; no RabbitMQ objects to reconcile",
			"source", src.Name,
			"predeclared queue", src.Spec.RabbitmqResourcesConfig.QueueName)
		return nil
	}

	_, err = r.rabbit.ReconcileExchange(ctx, &rabbit.ExchangeArgs{
		Name:                     naming.CreateSourceRabbitName(src),
		Namespace:                src.Namespace,
		RabbitmqClusterReference: src.Spec.RabbitmqClusterReference,
		RabbitMQVhost:            src.Spec.RabbitmqResourcesConfig.Vhost,
		Source:                   src,
	})
	if err != nil {
		logger.Error("failed to reconcile exchange", "exchange", src.Spec.RabbitmqResourcesConfig.ExchangeName)
		return err
	}

	_, err = r.rabbit.ReconcileQueue(ctx, &rabbit.QueueArgs{
		Name:                     naming.CreateSourceRabbitName(src),
		Namespace:                src.Namespace,
		RabbitmqClusterReference: src.Spec.RabbitmqClusterReference,
		RabbitMQVhost:            src.Spec.RabbitmqResourcesConfig.Vhost,
		Source:                   src,
		Owner:                    *kmeta.NewControllerRef(src),
		Labels:                   rabbit.Labels(nil, nil, src),
	})
	if err != nil {
		logger.Error("failed to reconcile queue", "queue", src.Spec.RabbitmqResourcesConfig.QueueName)
		return err
	}

	_, err = r.rabbit.ReconcileBinding(ctx, &rabbit.BindingArgs{
		Name:                     naming.CreateSourceRabbitName(src),
		Namespace:                src.Namespace,
		RabbitmqClusterReference: src.Spec.RabbitmqClusterReference,
		RabbitMQVhost:            src.Spec.RabbitmqResourcesConfig.Vhost,
		Source:                   src.Spec.RabbitmqResourcesConfig.ExchangeName,
		Destination:              src.Spec.RabbitmqResourcesConfig.QueueName,
		Owner:                    *kmeta.NewControllerRef(src),
		Labels:                   rabbit.Labels(nil, nil, src),
	})
	if err != nil {
		logger.Error("failed to reconcile queue", "queue", src.Spec.RabbitmqResourcesConfig.QueueName)
		return err
	}

	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.RabbitmqSource, sinkURI *apis.URL) (*v1.Deployment, error) {
	loggingConfig, err := logging.ConfigToJSON(r.loggingConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting logging config to JSON", zap.Any("receiveAdapter", err))
	}

	metricsConfig, err := metrics.OptionsToJSON(r.metricsConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting metrics config to JSON", zap.Any("receiveAdapter", err))
	}

	secretName, err := r.rabbit.GetRabbitMQCASecret(ctx, src.Spec.RabbitmqClusterReference)
	if err != nil {
		return nil, err
	}

	resourceRequirements, err := utils.GetResourceRequirements(src.ObjectMeta)
	if err != nil {
		return nil, err
	}

	raArgs := resources.ReceiveAdapterArgs{
		Image:                r.receiveAdapterImage,
		Source:               src,
		Labels:               resources.GetLabels(src.Name),
		SinkURI:              sinkURI,
		MetricsConfig:        metricsConfig,
		LoggingConfig:        loggingConfig,
		RabbitMQSecretName:   rabbit.SecretName(src.Name, "source"),
		RabbitMQCASecretName: secretName,
		BrokerUrlSecretKey:   rabbit.BrokerURLSecretKey,
		ResourceRequirements: resourceRequirements,
	}
	expected := resources.MakeReceiveAdapter(&raArgs)

	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Get(ctx, expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Issue eventing#2842: Adater deployment name uses kmeta.ChildName. If a deployment by the previous name pattern is found, it should
		// be deleted. This might cause temporary downtime.
		if deprecatedName := eventingutils.GenerateFixedName(raArgs.Source, fmt.Sprintf("rabbitmqsource-%s", raArgs.Source.Name)); deprecatedName != expected.Name {
			if err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Delete(ctx, deprecatedName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("error deleting deprecated named deployment: %v", err)
			}
			controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeNormal, rabbitmqSourceDeploymentDeleted, "Deprecated deployment removed: \"%s/%s\"", src.Namespace, deprecatedName)
		}
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, newDeploymentFailed(ra.Namespace, ra.Name, err)
		}
		return ra, newDeploymentCreated(ra.Namespace, ra.Name)
	} else if err != nil {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by RabbitmqSource %q", ra.Name, src.Name)
	} else if podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ctx, ra, metav1.UpdateOptions{}); err != nil {
			return ra, err
		}
		return ra, deploymentUpdated(ra.Namespace, ra.Name)
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
}

func podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
	if !equality.Semantic.DeepDerivative(newPodSpec, oldPodSpec) {
		return true
	}
	if len(oldPodSpec.Containers) != len(newPodSpec.Containers) {
		return true
	}
	for i := range newPodSpec.Containers {
		if !equality.Semantic.DeepEqual(newPodSpec.Containers[i].Env, oldPodSpec.Containers[i].Env) {
			return true
		}
	}
	return false
}

func (r *Reconciler) UpdateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	logcfg, err := logging.NewConfigFromConfigMap(cfg)
	if err != nil {
		logging.FromContext(r.loggingContext).Warn("failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}

	r.loggingConfig = logcfg
	logging.FromContext(r.loggingContext).Info("Update from logging ConfigMap", zap.Any("ConfigMap", cfg))
}

func (r *Reconciler) UpdateFromMetricsConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	r.metricsConfig = &metrics.ExporterOptions{
		Domain:    metrics.Domain(),
		Component: component,
		ConfigMap: cfg.Data,
	}
	logging.FromContext(r.loggingContext).Info("Update from metrics ConfigMap", zap.Any("ConfigMap", cfg))
}

func (r *Reconciler) createCloudEventAttributes(src *v1alpha1.RabbitmqSource) []duckv1.CloudEventAttributes {
	ceAttribute := duckv1.CloudEventAttributes{
		Type:   v1alpha1.RabbitmqEventType,
		Source: v1alpha1.RabbitmqEventSource(src.Namespace, src.Name, src.Spec.RabbitmqResourcesConfig.QueueName),
	}
	return []duckv1.CloudEventAttributes{ceAttribute}
}
