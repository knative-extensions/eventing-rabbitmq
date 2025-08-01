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

package rabbitmq

import (
	"context"
	"os"

	"knative.dev/eventing-rabbitmq/pkg/rabbit"
	bindinginformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/binding"
	exchangeinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/exchange"
	queueinformer "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/informers/rabbitmq.com/v1beta1/queue"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing-rabbitmq/pkg/apis/sources/v1alpha1"
	rabbitmqclient "knative.dev/eventing-rabbitmq/pkg/client/injection/client"
	rabbitmqinformer "knative.dev/eventing-rabbitmq/pkg/client/injection/informers/sources/v1alpha1/rabbitmqsource"
	"knative.dev/eventing-rabbitmq/pkg/client/injection/reconciler/sources/v1alpha1/rabbitmqsource"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher) *controller.Impl {

	raImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		logging.FromContext(ctx).Errorf("required environment variable '%s' not defined", raImageEnvVar)
		return nil
	}

	rabbitmqInformer := rabbitmqinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	bindingInformer := bindinginformer.Get(ctx)
	queueInformer := queueinformer.Get(ctx)
	exchangeInformer := exchangeinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)

	c := &Reconciler{
		KubeClientSet:       kubeclient.Get(ctx),
		secretLister:        secretInformer.Lister(),
		rabbitmqClientSet:   rabbitmqclient.Get(ctx),
		rabbitmqLister:      rabbitmqInformer.Lister(),
		deploymentLister:    deploymentInformer.Lister(),
		bindingListener:     bindingInformer.Lister(),
		exchangeLister:      exchangeInformer.Lister(),
		queueLister:         queueInformer.Lister(),
		receiveAdapterImage: raImage,
		loggingContext:      ctx,
		rabbit:              rabbit.New(ctx),
	}

	impl := rabbitmqsource.NewImpl(ctx, c)
	c.sinkResolver = resolver.NewURIResolverFromTracker(ctx, impl.Tracker)

	logging.FromContext(ctx).Info("Setting up rabbitmq event handlers")

	rabbitmqInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1alpha1.Kind("RabbitmqSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	bindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1alpha1.Kind("RabbitmqSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	queueInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1alpha1.Kind("RabbitmqSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	exchangeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1alpha1.Kind("RabbitmqSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1alpha1.Kind("RabbitmqSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	cmw.Watch(logging.ConfigMapName(), c.UpdateFromLoggingConfigMap)
	cmw.Watch(o11yconfigmap.Name(), c.UpdateFromObservabilityConfigMap)
	return impl
}
