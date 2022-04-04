# RabbitMQ Operator based Knative Eventing Broker

## Prerequisites

* Install Knative Eventing as documented [here](https://knative.dev/docs/install/).

* Install latest released version of the [RabbitMQ Cluster Operator](https://github.com/rabbitmq/cluster-operator).

    ```
    kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml
    ```

* Install latest released version of the [RabbitMQ Messaging Topology Operator](https://github.com/rabbitmq/messaging-topology-operator)

    Cert Manager is a pre-requisite, and we need to install it first:

    ```
    kubectl apply -f https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
    ```

    Now we can install the messaging topology operator:

    ```
    kubectl apply -f https://github.com/rabbitmq/messaging-topology-operator/releases/latest/download/messaging-topology-operator-with-certmanager.yaml
    ```

    If you already have Cert Manager installed, or want more control over the certs used, etc.
    You can follow the [quickstart here](https://github.com/rabbitmq/messaging-topology-operator#quickstart).

## Installation

You can install the latest released version of the [Operator based Knative RabbitMQ Broker](https://github.com/knative-sandbox/eventing-rabbitmq/releases/):

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/latest/download/rabbitmq-broker.yaml
```

If you wanted to install a specific version, e.g. v0.25.0, you can run:

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/download/v0.25.0/rabbitmq-broker.yaml
```

You can install a nightly version:

```shell
kubectl apply -f https://storage.googleapis.com/knative-nightly/eventing-rabbitmq/latest/rabbitmq-broker.yaml
```

Or if you want to run the latest version from this repo, you can use [`ko`](https://github.com/google/ko) to install it.

```
ko apply -f config/broker/
```

Before we can create the Knative Eventing Broker of type `RabbitMQBroker`, we first need to create a RabbitMQ Cluster:

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

And now we can create a Knative RabbitMQ Broker by executing the following command:

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

## Next steps

- Now that you have Knative Eventing integrated with RabbitMQ, a good next step is to use it with the [CloudEvents Player Source](https://knative.dev/docs/getting-started/first-source/) so that you can get a better understanding of how it all fits together.
