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

package e2e

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing-rabbitmq/test/e2e/config/certsecret"
	"knative.dev/eventing-rabbitmq/test/e2e/config/rabbitmq"
	kubeClient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

const (
	topologyOperatorDeploymentName = "messaging-topology-operator"
	volName                        = "eventing-rabbitmq-e2e-ca"
)

func SetupSelfSignedCerts() *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install self-signed certs as secrets", certsecret.Install)
	f.Setup("patch topology operator with rabbitmq-ca secret", PatchTopologyOperatorDeployment)
	f.Requirement("topology operator deployment is ready", TopologyOperatorDeploymentReady)

	return f
}

func CleanupSelfSignedCerts() *feature.Feature {
	f := new(feature.Feature)
	f.Teardown("clean up topology operator volumes and mounts", CleanUpTopologyOperatorVolumes)
	return f
}

func PatchTopologyOperatorDeployment(ctx context.Context, t feature.T) {
	namespace := environment.FromContext(ctx).Namespace()
	deployment, err := kubeClient.Get(ctx).AppsV1().Deployments("rabbitmq-system").Get(ctx, topologyOperatorDeploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Attach the new rabbitmq-ca certificate to the deployment in a volume
	vol := v1.Volume{
		Name: volName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: fmt.Sprintf("%s-rabbitmq-ca", namespace),
			},
		},
	}

	volFound := false
	for i, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == volName {
			volFound = true
			deployment.Spec.Template.Spec.Volumes[i] = vol
			break
		}
	}

	if !volFound {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, vol)
	}

	// Mount the new volume into the manager container
	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name != "manager" {
			continue
		}

		mountFound := false
		for _, v := range c.VolumeMounts {
			if v.Name == volName {
				mountFound = true
			}
		}
		if !mountFound {
			deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(c.VolumeMounts, v1.VolumeMount{
				MountPath: "/etc/ssl/certs/rabbitmq-ca.crt",
				Name:      volName,
				SubPath:   "ca.crt",
			})
		}
	}

	if _, err = kubeClient.Get(ctx).AppsV1().Deployments("rabbitmq-system").Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TopologyOperatorDeploymentReady(ctx context.Context, t feature.T) {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		deployment, err := kubeClient.Get(ctx).AppsV1().Deployments("rabbitmq-system").Get(ctx, topologyOperatorDeploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return deployment.Status.ReadyReplicas == deployment.Status.Replicas, nil
	})

	if err != nil {
		t.Fatal(err)
	}
}

func CleanUpTopologyOperatorVolumes(ctx context.Context, t feature.T) {
	deployment, err := kubeClient.Get(ctx).AppsV1().Deployments("rabbitmq-system").Get(ctx, topologyOperatorDeploymentName, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
	}

	for i, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == volName {
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes[:i], deployment.Spec.Template.Spec.Volumes[i+1:]...)
			break
		}
	}

	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name != "manager" {
			continue
		}

		for j, v := range c.VolumeMounts {
			if v.Name == volName {
				deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts[:j], deployment.Spec.Template.Spec.Containers[i].VolumeMounts[j+1:]...)
			}
		}
	}

	if _, err = kubeClient.Get(ctx).AppsV1().Deployments("rabbitmq-system").Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
		t.Error(err)
	}

	// Attempt to delete the ca secret
	namespace := environment.FromContext(ctx).Namespace()
	_ = kubeClient.Get(ctx).CoreV1().Secrets("rabbitmq-system").Delete(ctx, fmt.Sprintf("%s-%s", namespace, rabbitmq.CA_SECRET_NAME), metav1.DeleteOptions{})
}
