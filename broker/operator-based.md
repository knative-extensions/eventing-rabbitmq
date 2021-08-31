# RabbitMQ Operator based Knative Eventing Broker

## Prerequisites

* Install Knative Eventing as documented [here](https://knative.dev/docs/install/).

* Install version 1.8 of the [RabbitMQ Cluster Operator](https://github.com/rabbitmq/cluster-operator).

    ```
    kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/download/v1.8.2/cluster-operator.yml

    ```
* Install version 0.11.0 of the [RabbitMQ Messaging Topology Operator](https://github.com/rabbitmq/messaging-topology-operator/releases/tag/v0.8.0)

    Install Cert manager first:
    ```
    kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml
    ```

    Then operator:
    ```
    kubectl apply -f https://github.com/rabbitmq/messaging-topology-operator/releases/download/v0.11.0/messaging-topology-operator-with-certmanager.yaml
    ```

    If you already have Cert Manager installed, or want more control over the certs used, etc. You can follow the [quickstart here](https://github.com/rabbitmq/messaging-topology-operator#quickstart).

## Installation

You can install a released version of
[Operator based Knative RabbitMQ Broker](https://github.com/knative-sandbox/eventing-rabbitmq/releases/).

For example, if you wanted to install version v0.25.0 you would run:

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/download/v0.25.0/rabbitmq-broker.yaml
```

You can install a nightly version:

```shell
kubectl apply -f https://storage.googleapis.com/knative-nightly/eventing-rabbitmq/latest/rabbitmq-broker.yaml
```


Or if you want to run the latest version from this repo, you can use
[`ko`](https://github.com/google/ko) to install it.

```
ko apply -f config/broker/
```

## Creating Knative Eventing Broker

First create a RabbitMQ Cluster with the operator:

```shell
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rokn
  namespace: default
spec:
  replicas: 1
EOF
```

Then create Knative RabbitMQ Broker by executing the following command:

```shell
kubectl apply -f - << EOF
  apiVersion: eventing.knative.dev/v1
  kind: Broker
  metadata:
    name: default
    annotations:
      eventing.knative.dev/broker.class: RabbitMQBroker
  spec:
    config:
      apiVersion: rabbitmq.com/v1beta1
      kind: RabbitmqCluster
      name: rokn
EOF
```
