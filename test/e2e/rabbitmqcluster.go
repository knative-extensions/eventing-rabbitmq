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

package e2e

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"log"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/injection/clients/dynamicclient"

	"k8s.io/apimachinery/pkg/api/meta"

	"testing"

	"knative.dev/eventing-rabbitmq/test/e2e/config/rabbitmq"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

const (
	rabbitMQClusterName = "rabbitmqc"
	rabbitMQAPIVersion  = "rabbitmq.com/v1beta1"
	rabbitMQClusterKind = "RabbitmqCluster"
)

// RabbitMQCluster creates a rabbitmq.com/rabbitmqclusters cluster that the
// Broker under test will use. This assumes that the RabbitMQ Operator has
// already been instealled.
func RabbitMQCluster() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install a rabbitmqcluster", rabbitmq.Install())
	f.Requirement("RabbitMQCluster goes ready", RabbitMQClusterReady)
	return f
}

func RabbitMQClusterReady(ctx context.Context, t *testing.T) {
	namespace := environment.FromContext(ctx).Namespace()
	lastMsg := ""
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		conditions, err := getConditions(ctx, namespace)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Println(namespace, rabbitMQClusterName, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}

		// Rabbit does not have an overarching Ready condition, so spin through them all
		// and if any is False, then do not proceed.
		conditionCount := 0
		allReady := true
		for _, condition := range conditions {
			conditionCount++
			if condition.Status != corev1.ConditionTrue {
				allReady = false
				msg := fmt.Sprintf("%s/%s condition %s is not ready, %s: %s", namespace, rabbitMQClusterName, condition.Type, condition.Reason, condition.Message)
				if msg != lastMsg {
					log.Println(msg)
					lastMsg = msg
				}
				break
			}
		}
		return conditionCount > 0 && allReady, nil
	})
	if err != nil {
		conditions, condErr := getConditions(ctx, namespace)
		if condErr != nil {
			t.Errorf("Failed to get conditions for non-ready RabbitMQCluster %s/%s : %v", namespace, rabbitMQClusterName, err)
		} else {
			log.Printf("Conditions for non-ready RabbitMQCluster %s/%s:\n", namespace, rabbitMQClusterName)
			for _, condition := range conditions {
				log.Printf("%s/%s condition %s is: %s  %s: %s", namespace, rabbitMQClusterName, condition.Type, condition.Status, condition.Reason, condition.Message)
			}
		}
		t.Fatalf("RabbitMQCluster %s/%s did not become ready : %v", namespace, rabbitMQClusterName, err)
	}
	log.Printf("rabbitmqcluster %s/%s is ready\n", namespace, rabbitMQClusterName)
}

func getConditions(ctx context.Context, namespace string) ([]apis.Condition, error) {
	rabbitCluster := corev1.ObjectReference{
		Namespace:  namespace,
		Name:       rabbitMQClusterName,
		APIVersion: rabbitMQAPIVersion,
		Kind:       rabbitMQClusterKind,
	}

	k := rabbitCluster.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(k)

	client := dynamicclient.Get(ctx)

	like := &duckv1.KResource{}
	us, err := client.Resource(gvr).Namespace(namespace).Get(context.Background(), rabbitMQClusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	obj := like.DeepCopy()
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, obj); err != nil {
		log.Fatal("Error DefaultUnstructuree.Dynamiconverter ")
	}
	obj.ResourceVersion = gvr.Version
	obj.APIVersion = gvr.GroupVersion().String()
	return obj.Status.GetConditions(), nil
}
