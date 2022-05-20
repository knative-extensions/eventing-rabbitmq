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
	"log"

	rabbitv1 "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	rabbitmqclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	"knative.dev/eventing-rabbitmq/pkg/brokerconfig"
	bindinginformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/binding"
	exchangeinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/exchange"
	policyinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/policy"
	queueinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/queue"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
	endpointsinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret"
	serviceinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/service"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

const (
	ComponentName = "rabbitmq-broker-controller"
)

type envConfig struct {
	IngressImage          string `envconfig:"BROKER_INGRESS_IMAGE" required:"true"`
	IngressServiceAccount string `envconfig:"BROKER_INGRESS_SERVICE_ACCOUNT" required:"true"`

	// Which image to use for the DeadLetter dispatcher
	DispatcherImage string `envconfig:"BROKER_DLQ_DISPATCHER_IMAGE" required:"true"`
}

const BrokerClass = "RabbitMQBroker"

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	eventingv1.RegisterAlternateBrokerConditionSet(rabbitBrokerCondSet)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	secretInformer := secretinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	brokerInformer := brokerinformer.Get(ctx)
	serviceInformer := serviceinformer.Get(ctx)
	endpointsInformer := endpointsinformer.Get(ctx)
	rabbitInformer := rabbitv1.Get(ctx)
	exchangeInformer := exchangeinformer.Get(ctx)
	policyInformer := policyinformer.Get(ctx)
	queueInformer := queueinformer.Get(ctx)
	bindingInformer := bindinginformer.Get(ctx)
	configmapInformer := configmapinformer.Get(ctx)

	r := &Reconciler{
		eventingClientSet:         eventingclient.Get(ctx),
		kubeClientSet:             kubeclient.Get(ctx),
		brokerLister:              brokerInformer.Lister(),
		secretLister:              secretInformer.Lister(),
		serviceLister:             serviceInformer.Lister(),
		endpointsLister:           endpointsInformer.Lister(),
		deploymentLister:          deploymentInformer.Lister(),
		rabbitLister:              rabbitInformer,
		ingressImage:              env.IngressImage,
		ingressServiceAccountName: env.IngressServiceAccount,
		brokerClass:               BrokerClass,
		dispatcherImage:           env.DispatcherImage,
		rabbitClientSet:           rabbitmqclient.Get(ctx),
		configs:                   reconcilersource.WatchConfigurations(ctx, ComponentName, cmw),
		rabbit:                    rabbit.New(ctx),
		brokerConfig:              brokerconfig.New(ctx),
	}

	impl := brokerreconciler.NewImpl(ctx, r, BrokerClass)

	logging.FromContext(ctx).Info("Setting up event handlers")

	r.kresourceTracker = duck.NewListableTrackerFromTracker(ctx, conditions.Get, impl.Tracker)
	r.addressableTracker = duck.NewListableTrackerFromTracker(ctx, addressable.Get, impl.Tracker)
	r.uriResolver = resolver.NewURIResolverFromTracker(ctx, impl.Tracker)

	brokerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.AnnotationFilterFunc(brokerreconciler.ClassAnnotationKey, BrokerClass, false /*allowUnset*/),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.LabelExistsFilterFunc(eventing.BrokerLabelKey),
		Handler:    controller.HandleAll(impl.EnqueueLabelOfNamespaceScopedResource("" /*any namespace*/, eventing.BrokerLabelKey)),
	})

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	exchangeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	policyInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	queueInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	bindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	configmapInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: utils.SystemConfigMapsFilterFunc(),
		Handler:    controller.HandleAll(func(interface{}) { impl.GlobalResync(brokerInformer.Informer()) }),
	})
	return impl
}
