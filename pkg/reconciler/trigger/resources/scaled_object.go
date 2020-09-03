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

package resources

import (
	kedav1alpha1 "github.com/kedacore/keda/pkg/apis/keda/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/kmeta"
)

type DispatcherScaledObjectArgs struct {
	DispatcherDeployment      *v1.Deployment
	QueueName                 string
	Trigger                   *v1beta1.Trigger
	BrokerUrlSecretKey        string
	TriggerAuthenticationName string
}

func MakeDispatcherScaledObject(args *DispatcherScaledObjectArgs) *kedav1alpha1.ScaledObject {
	zero := int32(0)
	one := int32(1)
	five := int32(5)
	thirty := int32(30)

	deployment := args.DispatcherDeployment
	return &kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
			Labels:    deployment.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Trigger),
			},
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ObjectReference{
				DeploymentName: deployment.Name,
			},
			PollingInterval: &five,   // seconds
			CooldownPeriod:  &thirty, // seconds
			MinReplicaCount: &zero,
			MaxReplicaCount: &one, // for now
			Triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "rabbitmq",
					Metadata: map[string]string{
						"queueName":   args.QueueName,
						"host":        args.BrokerUrlSecretKey,
						"queueLength": "1",
					},
					AuthenticationRef: &kedav1alpha1.ScaledObjectAuthRef{
						Name: args.TriggerAuthenticationName,
					},
				},
			},
		},
	}
}
