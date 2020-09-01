/*
Copyright 2019 The Rigging Authors

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

package lifecycle

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kntest "knative.dev/pkg/test"
)

// Client holds instances of interfaces for making requests to Kubernetes.
type Client struct {
	Kube             *kntest.KubeClient
	Dynamic          dynamic.Interface
	Namespace        string
	namespaceCreated bool
}

// NewClient instantiates and returns clientsets required for making request to the
// cluster specified by the combination of clusterName and configPath.
func NewClient(configPath string, clusterName string, namespace string) (*Client, error) {
	client := &Client{}
	cfg, err := kntest.BuildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}
	client.Kube, err = kntest.NewKubeClient(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	client.Dynamic, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	client.Namespace = namespace
	client.namespaceCreated = true
	return client, nil
}

// CreateNamespaceIfNeeded creates a new namespace if it does not exist.
func (c *Client) CreateNamespaceIfNeeded() error {
	nsSpec, err := c.Kube.Kube.CoreV1().Namespaces().Get(c.Namespace, metav1.GetOptions{})

	if err != nil && apierrors.IsNotFound(err) {
		nsSpec = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: c.Namespace}}
		nsSpec, err = c.Kube.Kube.CoreV1().Namespaces().Create(nsSpec)

		if err != nil {
			return fmt.Errorf("Failed to create Namespace: %s; %v", c.Namespace, err)
		}

		// https://github.com/kubernetes/kubernetes/issues/66689
		// We can only start creating pods after the default ServiceAccount is created by the kube-controller-manager.
		err = waitForServiceAccountExists(c, "default", c.Namespace)
		if err != nil {
			return fmt.Errorf("The default ServiceAccount was not created for the Namespace: %s", c.Namespace)
		}
	}
	return nil
}

func (c *Client) DeleteNamespaceIfNeeded() error {
	if c.namespaceCreated {
		_, err := c.Kube.Kube.CoreV1().Namespaces().Get(c.Namespace, metav1.GetOptions{})
		if err == nil || !apierrors.IsNotFound(err) {
			return c.Kube.Kube.CoreV1().Namespaces().Delete(c.Namespace, nil)
		}
		return err
	}
	return nil
}

// DuplicateSecret duplicates a secret from a namespace to a new namespace.
func (c *Client) DuplicateSecret(t *testing.T, name, namespace string) {
	secret, err := c.Kube.Kube.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to find Secret: %q in Namespace: %q: %s", name, namespace, err)
		return
	}
	newSecret := &corev1.Secret{}
	newSecret.Name = name
	newSecret.Namespace = c.Namespace
	newSecret.Data = secret.Data
	newSecret.StringData = secret.StringData
	newSecret.Type = secret.Type
	newSecret, err = c.Kube.Kube.CoreV1().Secrets(c.Namespace).Create(newSecret)
	if err != nil {
		t.Fatalf("Failed to create Secret: %s; %v", c.Namespace, err)
	}
}

const (
	interval = 1 * time.Second
	//timeout  = 2 * time.Minute // TODO: change this to be configurable.
	timeout = 45 * time.Second
)

// waitForServiceAccountExists waits until the ServiceAccount exists.
func waitForServiceAccountExists(client *Client, name, namespace string) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		sas := client.Kube.Kube.CoreV1().ServiceAccounts(namespace)
		if _, err := sas.Get(name, metav1.GetOptions{}); err == nil {
			return true, nil
		}
		return false, nil
	})
}

// WaitForResourceReady waits until the specified resource in the given namespace are ready.
func (c *Client) WaitForResourceReady(namespace, name string, gvr schema.GroupVersionResource, timeout time.Duration) error {
	lastMsg := ""
	like := &duckv1.KResource{}
	return wait.PollImmediate(interval, timeout, func() (bool, error) {

		us, err := c.Dynamic.Resource(gvr).Namespace(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Println(namespace, name, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		obj := like.DeepCopy()
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, obj); err != nil {
			log.Fatalf("Error DefaultUnstructuree.Dynamiconverter. %v", err)
		}
		obj.ResourceVersion = gvr.Version
		obj.APIVersion = gvr.GroupVersion().String()

		ready := obj.Status.GetCondition(apis.ConditionReady)
		if ready != nil && !ready.IsTrue() {
			msg := fmt.Sprintf("%s is not ready, %s: %s", name, ready.Reason, ready.Message)
			if msg != lastMsg {
				log.Println(msg)
				lastMsg = msg
			}
		}

		return ready.IsTrue(), nil
	})
}

// WaitForResourceReady waits until the specified resource in the given namespace are ready.
func (c *Client) WaitUntilJobDone(namespace, name string, timeout time.Duration) (string, error) {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		job, err := c.Kube.Kube.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Println(namespace, name, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		return IsJobComplete(job), nil
	})
	if err != nil {
		return "", err
	}

	// poll until the pod is terminated.
	err = wait.PollImmediate(interval, timeout, func() (bool, error) {
		pod, err := GetJobPodByJobName(context.TODO(), c.Kube.Kube, namespace, name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Println(namespace, name, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		if pod != nil {
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Terminated != nil {
					return true, nil
				}
			}
		}
		return false, nil
	})

	if err != nil {
		return "", err
	}
	pod, err := GetJobPodByJobName(context.TODO(), c.Kube.Kube, namespace, name)
	if err != nil {
		return "", err
	}
	return GetFirstTerminationMessage(pod), nil
}

func (c *Client) LogsFor(namespace, name string, gvr schema.GroupVersionResource) (string, error) {
	// Get all pods in this namespace.
	pods, err := c.Kube.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	logs := make([]string, 0)

	// Look for a pod with the name that was passed in inside the pod name.
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, name) {
			// Collect all the logs from all the containers for this pod.
			if l, err := c.Kube.Kube.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{}).DoRaw(); err != nil {
				logs = append(logs, err.Error())
			} else {
				logs = append(logs, string(l))
			}
		}
	}

	// Did we find a match like the given name?
	if len(logs) == 0 {
		return "", fmt.Errorf(`pod for "%s/%s" [%s] not found`, namespace, name, gvr.String())
	}

	return strings.Join(logs, "\n"), nil
}
