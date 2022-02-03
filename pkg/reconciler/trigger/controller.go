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
	"log"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	rabbitmqclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"

	bindinginformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/binding"
	exchangeinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/exchange"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/source"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"

	policyinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/policy"
	queueinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/queue"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

const (
	BrokerClass = "RabbitMQBroker"
	component   = "rabbitmq-trigger-controller"
)

type envConfig struct {
	DispatcherImage          string `envconfig:"BROKER_DISPATCHER_IMAGE" required:"true"`
	DispatcherServiceAccount string `envconfig:"BROKER_DISPATCHER_SERVICE_ACCOUNT" required:"true"`
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	brokerInformer := brokerinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	triggerInformer := triggerinformer.Get(ctx)
	policyInformer := policyinformer.Get(ctx)
	queueInformer := queueinformer.Get(ctx)
	bindingInformer := bindinginformer.Get(ctx)
	exchangeInformer := exchangeinformer.Get(ctx)
	configmapInformer := configmapinformer.Get(ctx)

	r := &Reconciler{
		eventingClientSet:            eventingclient.Get(ctx),
		dynamicClientSet:             dynamicclient.Get(ctx),
		kubeClientSet:                kubeclient.Get(ctx),
		deploymentLister:             deploymentInformer.Lister(),
		brokerLister:                 brokerInformer.Lister(),
		triggerLister:                triggerInformer.Lister(),
		dispatcherImage:              env.DispatcherImage,
		dispatcherServiceAccountName: env.DispatcherServiceAccount,
		brokerClass:                  BrokerClass,
		exchangeLister:               exchangeInformer.Lister(),
		queueLister:                  queueInformer.Lister(),
		bindingLister:                bindingInformer.Lister(),
		rabbitClientSet:              rabbitmqclient.Get(ctx),
		configs:                      reconcilersource.WatchConfigurations(ctx, component, cmw),
		rabbit:                       rabbit.New(ctx),
	}

	impl := triggerreconciler.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers")

	r.sourceTracker = duck.NewListableTrackerFromTracker(ctx, source.Get, impl.Tracker)
	r.addressableTracker = duck.NewListableTrackerFromTracker(ctx, addressable.Get, impl.Tracker)
	r.uriResolver = resolver.NewURIResolverFromTracker(ctx, impl.Tracker)

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1.Kind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	queueInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1.Kind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	policyInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1.Kind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	exchangeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1.Kind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	bindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1.Kind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	configmapInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: utils.SystemConfigMapsFilterFunc(),
		Handler:    controller.HandleAll(func(interface{}) { impl.GlobalResync(triggerInformer.Informer()) }),
	})

	brokerInformer.Informer().AddEventHandler(controller.HandleAll(
		func(obj interface{}) {
			if broker, ok := obj.(*v1.Broker); ok {
				triggers, err := triggerInformer.Lister().Triggers(broker.Namespace).List(labels.Everything())
				if err != nil {
					log.Print("Failed to lookup Triggers for Broker", zap.Error(err))
				} else {
					for _, t := range triggers {
						if t.Spec.Broker == broker.Name {
							impl.Enqueue(t)
						}
					}
				}
			}
		},
	))

	triggerInformer.Informer().AddEventHandler(controller.HandleAll(
		func(obj interface{}) {
			if trigger, ok := obj.(*v1.Trigger); ok {
				broker, err := brokerInformer.Lister().Brokers(trigger.Namespace).Get(trigger.Spec.Broker)
				if err != nil {
					log.Print("Failed to lookup Broker for Trigger", zap.Error(err))
				} else {
					label := broker.ObjectMeta.Annotations[brokerreconciler.ClassAnnotationKey]
					if label == BrokerClass {
						impl.Enqueue(obj)
					}
				}
			}
		},
	))
	return impl
}
