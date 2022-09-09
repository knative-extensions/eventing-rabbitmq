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

package rabbit

import (
	"context"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	rabbitclusterv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	rabbitduck "knative.dev/eventing-rabbitmq/pkg/client/injection/ducks/duck/v1beta1/rabbit"
	"knative.dev/eventing-rabbitmq/third_party/pkg/apis/rabbitmq.com/v1beta1"
	fakerabbitclient "knative.dev/eventing-rabbitmq/third_party/pkg/client/injection/client/fake"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/injection"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/kmeta"
)

func Test_NewService(t *testing.T) {
	ctx := context.TODO()
	ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})
	ctx, _ = fakedynamicclient.With(ctx, runtime.NewScheme())
	ctx = rabbitduck.WithDuck(ctx)
	ctx, fakekubeClientSet := fakekubeclient.With(ctx)

	i := fakerabbitclient.Get(ctx)
	want := &Rabbit{Interface: i, rabbitLister: rabbitduck.Get(ctx), kubeClientSet: fakekubeClientSet}
	got := New(ctx)
	if want.Interface != got.Interface {
		t.Errorf("New function did not return a valid Rabbit interface %s %s", want, got)
	}
}

func Test_isReadyFunc(t *testing.T) {
	for _, tt := range []struct {
		name       string
		conditions []v1beta1.Condition
		want       bool
	}{{
		name: "Nil condition set",
		want: false,
	}, {
		name:       "Empty condition set",
		conditions: []v1beta1.Condition{},
		want:       false,
	}, {
		name:       "Valid condition set",
		conditions: []v1beta1.Condition{{Status: v1.ConditionTrue}, {Status: v1.ConditionTrue}},
		want:       true,
	}, {
		name:       "Invalid condition set",
		conditions: []v1beta1.Condition{{Status: v1.ConditionTrue}, {Status: v1.ConditionFalse}},
		want:       false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			got := isReady(tt.conditions)
			if got != tt.want {
				t.Errorf("unexpected error checking conditions want: %v, got: %v", tt.want, got)
			}
		})
	}
}

func Test_RabbitMQURL(t *testing.T) {
	ctx := context.TODO()
	ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})
	ctx, _ = fakedynamicclient.With(ctx, runtime.NewScheme())
	ctx = rabbitduck.WithDuck(ctx)
	ctx, fakekubeClientSet := fakekubeclient.With(ctx)
	i := fakerabbitclient.Get(ctx)
	r := &Rabbit{Interface: i, rabbitLister: rabbitduck.Get(ctx), kubeClientSet: fakekubeClientSet}
	for i, tt := range []struct {
		name, wantUrl      string
		secretData         map[string][]byte
		conSecret, wantErr bool
	}{{
		name:    "connection secret not available",
		wantErr: true,
	}, {
		name:      "missing uri connection secret",
		conSecret: true,
		wantErr:   true,
	}, {
		name:       "missing password connection secret",
		conSecret:  true,
		secretData: map[string][]byte{"uri": []byte("test-uri")},
		wantErr:    true,
	}, {
		name:       "missing username connection secret",
		conSecret:  true,
		secretData: map[string][]byte{"uri": []byte("test-uri"), "password": []byte("1234")},
		wantErr:    true,
	}, {
		name:       "valid connection secret",
		conSecret:  true,
		secretData: map[string][]byte{"uri": []byte("test-uri"), "password": []byte("1234"), "username": []byte("test")},
		wantUrl:    "amqp://test:1234@test-uri:5672",
		wantErr:    false,
	}, {
		name:       "https with custom port secret",
		conSecret:  true,
		secretData: map[string][]byte{"uri": []byte("https://test-uri"), "password": []byte("1234"), "username": []byte("test"), "port": []byte("1234")},
		wantUrl:    "amqps://test:1234@https:1234",
		wantErr:    false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			i := i
			t.Parallel()
			var name string
			var s *v1.Secret
			b := CreateBroker("test-broker", "default")
			if !tt.conSecret {
				name = "non-existing-secret"
			} else {
				s = CreateSecret(fmt.Sprint(i), "broker", "default", "test-url", b, tt.secretData)
				r.kubeClientSet.CoreV1().Secrets("default").Create(ctx, s, metav1.CreateOptions{})
				name = s.Name
			}
			cr := &v1beta1.RabbitmqClusterReference{
				Name:             fmt.Sprintf("test-%d", i),
				Namespace:        "default",
				ConnectionSecret: &v1.LocalObjectReference{Name: name},
			}

			gotUrl, err := r.RabbitMQURL(ctx, cr)
			if (err != nil && !tt.wantErr) || (err == nil && tt.wantErr) {
				t.Errorf("unexpected error checking conditions want: %v, got: %v", tt.wantErr, err)
			} else if !tt.wantErr && gotUrl.String() != tt.wantUrl {
				t.Errorf("got wrong url want: %s, got: %s", tt.wantUrl, gotUrl)
			}
		})
	}
}

func TestGetRabbitMQCASecret(t *testing.T) {
	for _, tt := range []struct {
		name, wantSecret string
		clusterRef       *v1beta1.RabbitmqClusterReference
		secret           *v1.Secret
		wantErr          bool
	}{{
		name: "errors when connection secret not available",
		clusterRef: &v1beta1.RabbitmqClusterReference{
			ConnectionSecret: &v1.LocalObjectReference{Name: "secret"},
		},
		wantErr: true,
	}, {
		name:       "errors for nil clusterRef",
		clusterRef: nil,
		wantErr:    true,
	}, {
		name: "no error, empty secretName when secret doesn't have correct field set",
		clusterRef: &v1beta1.RabbitmqClusterReference{
			Namespace: "default",
			ConnectionSecret: &v1.LocalObjectReference{
				Name: "secret-name",
			},
		},
		secret: &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-name",
				Namespace: "default",
			},
		},
		wantErr: false,
	}, {
		name: "no error, correct secret name when clusterRef is a secret",
		clusterRef: &v1beta1.RabbitmqClusterReference{
			Namespace: "default",
			ConnectionSecret: &v1.LocalObjectReference{
				Name: "secret-name",
			},
		},
		secret: &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-name",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"caSecretName": []byte("some-secret-name"),
			},
		},
		wantErr:    false,
		wantSecret: "some-secret-name",
	}, {
		name: "errors if RabbitMQCluster not found",
		clusterRef: &v1beta1.RabbitmqClusterReference{
			Namespace: "default",
			Name:      "some-cluster",
		},
		wantErr: true,
	}} {
		// TODO: add more RabbitmqCluster tests
		t.Run(tt.name, func(t *testing.T) {
			tt := tt

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})
			scheme := runtime.NewScheme()
			rabbitclusterv1beta1.AddToScheme(scheme)
			ctx, _ = fakedynamicclient.With(ctx, scheme)
			ctx = rabbitduck.WithDuck(ctx)
			ctx, fakekubeClientSet := fakekubeclient.With(ctx)
			i := fakerabbitclient.Get(ctx)

			if tt.secret != nil {
				_, err := fakekubeClientSet.CoreV1().Secrets(tt.clusterRef.Namespace).Create(ctx, tt.secret, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("failed to create secret: %s", err)
				}
			}

			r := &Rabbit{Interface: i, rabbitLister: rabbitduck.Get(ctx), kubeClientSet: fakekubeClientSet}

			gotSecretName, err := r.GetRabbitMQCASecret(ctx, tt.clusterRef)
			if (err != nil && !tt.wantErr) || (err == nil && tt.wantErr) {
				t.Errorf("unexpected error checking conditions want: %v, got: %v", tt.wantErr, err)
			}
			if tt.wantSecret != gotSecretName {
				t.Errorf("unexpected secretname: want: %s, got: %s", tt.wantSecret, gotSecretName)
			}
		})
	}
}

func CreateBroker(name, namespace string) *eventingv1.Broker {
	b := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	b.SetDefaults(context.Background())
	return b
}

func CreateSecret(name, typeString, namespace, url string, owner kmeta.OwnerRefable, secretData map[string][]byte) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      SecretName(name, typeString),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(owner),
			},
			Labels: SecretLabels(name, typeString),
		},
		Data: secretData,
	}
}
