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

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"

	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta1/broker"
	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta1/trigger"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/broker"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/trigger"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

type envConfig struct {
	DispatcherImage          string `envconfig:"BROKER_DISPATCHER_IMAGE" required:"true"`
	DispatcherServiceAccount string `envconfig:"BROKER_DISPATCHER_SERVICE_ACCOUNT" required:"true"`

	// The default value should match the cluster default in config/core/configmaps/default-broker.yaml
	BrokerClass string `envconfig:"BROKER_CLASS" default:"RabbitMQBroker"`
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

	r := &Reconciler{
		eventingClientSet:            eventingclient.Get(ctx),
		dynamicClientSet:             dynamicclient.Get(ctx),
		kubeClientSet:                kubeclient.Get(ctx),
		deploymentLister:             deploymentInformer.Lister(),
		brokerLister:                 brokerInformer.Lister(),
		triggerLister:                triggerInformer.Lister(),
		dispatcherImage:              env.DispatcherImage,
		dispatcherServiceAccountName: env.DispatcherServiceAccount,
		brokerClass:                  env.BrokerClass,
	}

	impl := triggerreconciler.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers")

	r.kresourceTracker = duck.NewListableTracker(ctx, conditions.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.addressableTracker = duck.NewListableTracker(ctx, addressable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.uriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1beta1.Kind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	brokerInformer.Informer().AddEventHandler(controller.HandleAll(
		func(obj interface{}) {
			if broker, ok := obj.(*v1beta1.Broker); ok {
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
			if trigger, ok := obj.(*v1beta1.Trigger); ok {
				broker, err := brokerInformer.Lister().Brokers(trigger.Namespace).Get(trigger.Spec.Broker)
				if err != nil {
					log.Print("Failed to lookup Broker for Trigger", zap.Error(err))
				} else {
					label := broker.ObjectMeta.Annotations[brokerreconciler.ClassAnnotationKey]
					if label == env.BrokerClass {
						impl.Enqueue(obj)
					}
				}
			}
		},
	))

	return impl
}
