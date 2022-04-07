# RabbitMQ Standalone Knative Eventing Broker

## Prerequisites

Install Knative Eventing as documented [here](https://knative.dev/docs/install/).

## Installation

You can install a released version of
[Standalone Knative RabbitMQ Broker](https://github.com/knative-sandbox/eventing-rabbitmq/releases/) starting in release v0.24 (Released in 2021-06-28).

For example, if you wanted to install version v0.24.0 you would run:

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/download/v0.24.0/rabbitmq-standalone-broker.yaml
```

Until release v0.24 goes out, you can install a nightly version:

```shell
kubectl apply -f https://storage.googleapis.com/knative-nightly/eventing-rabbitmq/latest/rabbitmq-standalone-broker.yaml
```

Or if you want to run the very latest version from this repo, you can use
[`ko`](https://github.com/google/ko) to install it.

```
ko apply -f config/brokerstandalone/
```

## Creating Knative Eventing Broker

You need to provide a kubernetes secret that the Broker will use to create the RabbitMQ resources (Exchanges, Queues, Bindings) as well as consume events from it. The secret must have a brokerURL key, and the value is the [amqp connection string](https://www.rabbitmq.com/uri-spec.html).

For example, if your connection string is `amqp://myusername:mypassword@myrabbitserver:5672` you could create the secret like this:

```
kubectl create secret generic rokn-rabbitmq-broker-secret \
--from-literal=brokerURL="amqp://myusername:mypassword@myrabbitserver:5672"
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
      apiVersion: v1
      kind: Secret
      name: rokn-rabbitmq-broker-secret
EOF
```

