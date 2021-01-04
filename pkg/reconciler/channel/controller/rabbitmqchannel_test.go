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
	"knative.dev/eventing-rabbitmq/pkg/client/injection/reconciler/messaging/v1beta1/rabbitmqchannel"
	"testing"

	"knative.dev/pkg/network"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	. "knative.dev/pkg/reconciler/testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/eventing-rabbitmq/pkg/apis/messaging/v1beta1"
	fakeclientset "knative.dev/eventing-rabbitmq/pkg/client/injection/client/fake"
	"knative.dev/eventing-rabbitmq/pkg/reconciler/channel/controller/resources"
	reconciletesting "knative.dev/eventing-rabbitmq/pkg/reconciler/testing"
)

const (
	testNS                   = "test-namespace"
	ncName                   = "test-nc"
	dispatcherDeploymentName = "test-deployment"
	dispatcherServiceName    = "test-service"
	channelServiceAddress    = "test-nc-kn-channel.test-namespace.svc.cluster.local"
)

func init() {
	// Add types to scheme
	_ = v1beta1.AddToScheme(scheme.Scheme)
	_ = duckv1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	ncKey := testNS + "/" + ncName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "deleting",
			Key:  ncKey,
			Objects: []runtime.Object{
				reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeleted)},
		}, {
			Name: "deployment does not exist",
			Key:  ncKey,
			Objects: []runtime.Object{
				reconciletesting.NewRabbitmqChannel(ncName, testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeploymentNotReady(dispatcherDeploymentNotFound, "Dispatcher Deployment does not exist"),
					reconciletesting.WithRabbitmqChannelChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelAddress(channelServiceAddress),
					reconciletesting.Addressable(),
					reconciletesting.WithRabbitmqChannelServiceNotReady(dispatcherServiceNotFound, "Dispatcher Service does not exist"),
					reconciletesting.WithRabbitmqChannelEndpointsNotReady(dispatcherEndpointsNotFound, "Dispatcher Endpoints does not exist"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewRabbitmqChannel(ncName, testNS)),
			},
		}, {
			Name: "Service does not exist",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				reconciletesting.NewRabbitmqChannel(ncName, testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeploymentReady(),
					reconciletesting.WithRabbitmqChannelChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelAddress(channelServiceAddress),
					reconciletesting.Addressable(),
					reconciletesting.WithRabbitmqChannelServiceNotReady(dispatcherServiceNotFound, "Dispatcher Service does not exist"),
					reconciletesting.WithRabbitmqChannelEndpointsNotReady(dispatcherEndpointsNotFound, "Dispatcher Endpoints does not exist"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewRabbitmqChannel(ncName, testNS)),
			},
		}, {
			Name: "Endpoints does not exist",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				reconciletesting.NewRabbitmqChannel(ncName, testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeploymentReady(),
					reconciletesting.WithRabbitmqChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelAddress(channelServiceAddress),
					reconciletesting.Addressable(),
					reconciletesting.WithRabbitmqChannelEndpointsNotReady(dispatcherEndpointsNotFound, "Dispatcher Endpoints does not exist"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewRabbitmqChannel(ncName, testNS)),
			},
		}, {
			Name: "Endpoints not ready",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeEmptyEndpoints(),
				reconciletesting.NewRabbitmqChannel(ncName, testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeploymentReady(),
					reconciletesting.WithRabbitmqChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelAddress(channelServiceAddress),
					reconciletesting.Addressable(),
					reconciletesting.WithRabbitmqChannelEndpointsNotReady("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewRabbitmqChannel(ncName, testNS)),
			},
		}, {
			Name: "Works, creates new channel",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewRabbitmqChannel(ncName, testNS),
			},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewRabbitmqChannel(ncName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeploymentReady(),
					reconciletesting.WithRabbitmqChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelEndpointsReady(),
					reconciletesting.WithRabbitmqChannelChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelAddress(channelServiceAddress),
					reconciletesting.Addressable(),
				),
			}},
		}, {
			Name: "Works, channel exists",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewRabbitmqChannel(ncName, testNS),
				makeChannelService(reconciletesting.NewRabbitmqChannel(ncName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeploymentReady(),
					reconciletesting.WithRabbitmqChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelEndpointsReady(),
					reconciletesting.WithRabbitmqChannelChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelAddress(channelServiceAddress),
				),
			}},
		}, {
			Name: "channel exists, not owned by us",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewRabbitmqChannel(ncName, testNS),
				makeChannelServiceNotOwnedByUs(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeploymentReady(),
					reconciletesting.WithRabbitmqChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelEndpointsReady(),
					reconciletesting.WithRabbitmqChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: natsschannel: test-namespace/test-nc does not own Service: \"test-nc-kn-channel\""),
				),
			}},
		}, {
			Name: "channel does not exist, fails to create",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewRabbitmqChannel(ncName, testNS),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "Services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewRabbitmqChannel(ncName, testNS,
					reconciletesting.WithRabbitmqInitChannelConditions,
					reconciletesting.WithRabbitmqChannelDeploymentReady(),
					reconciletesting.WithRabbitmqChannelServiceReady(),
					reconciletesting.WithRabbitmqChannelEndpointsReady(),
					reconciletesting.WithRabbitmqChannelChannelServicetNotReady(channelServiceFailed, "Channel Service failed: inducing failure for create services"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewRabbitmqChannel(ncName, testNS)),
			},
		},
	}

	table.Test(t, reconciletesting.MakeFactory(func(ctx context.Context, listers *reconciletesting.Listers) controller.Reconciler {
		r := &Reconciler{
			dispatcherNamespace:      testNS,
			dispatcherDeploymentName: dispatcherDeploymentName,
			dispatcherServiceName:    dispatcherServiceName,
			kubeClientSet:            fakekubeclient.Get(ctx),
			deploymentLister:         listers.GetDeploymentLister(),
			serviceLister:            listers.GetServiceLister(),
			endpointsLister:          listers.GetEndpointsLister(),
		}
		return rabbitmqchannel.NewReconciler(ctx, logging.FromContext(ctx),
			fakeclientset.Get(ctx), listers.GetNatssChannelLister(),
			controller.GetEventRecorder(ctx),
			r)
	}))
}

func makeDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      dispatcherDeploymentName,
		},
		Status: appsv1.DeploymentStatus{},
	}
}

func makeReadyDeployment() *appsv1.Deployment {
	d := makeDeployment()
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
	return d
}

func makeService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      dispatcherServiceName,
		},
	}
}

func makeChannelService(nc *v1beta1.RabbitmqChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", ncName),
			Labels: map[string]string{
				resources.MessagingRoleLabel: resources.MessagingRole,
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(nc),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: network.GetServiceHostname(dispatcherServiceName, testNS),
		},
	}
}

func makeChannelServiceNotOwnedByUs() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", ncName),
			Labels: map[string]string{
				resources.MessagingRoleLabel: resources.MessagingRole,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: network.GetServiceHostname(dispatcherServiceName, testNS),
		},
	}
}

func makeEmptyEndpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Endpoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      dispatcherServiceName,
		},
	}
}

func makeReadyEndpoints() *corev1.Endpoints {
	e := makeEmptyEndpoints()
	e.Subsets = []corev1.EndpointSubset{{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}}}
	return e
}
