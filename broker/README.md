# RabbitMQ Knative Eventing Broker

![RabbitMQ Broker for Knative Eventing](rabbitmq-knative-broker.png)

## Prerequisites

install Knative Serving and Eventing as documented
[here](https://knative.dev/docs/install/any-kubernetes-cluster/)

Install the RabbitMQ Operator as documented. At least version v1.0.0 is
required. If you have an existing RabbitMQ and want to use that, please use the
(existing RabbitMQ instructions)[#existing-rabbitmq].
[here](https://github.com/rabbitmq/cluster-operator).

## Installation

You can install a released version of [Knative RabbitMQ
Broker](https://github.com/knative-sandbox/eventing-rabbitmq/releases/)

For example, if you wanted to install version v0.19.0 you would run:

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/download/v0.19.0/rabbitmq-broker.yaml
```

If you want to run the latest version from this repo, you can use
[`ko`](https://github.com/google/ko) to install it.

```
ko apply -f config/broker/
```

## Creating a Knative RabbitMQ Broker

### Creating a Knative RabbitMQ Broker using RabbitMQ cluster operator

You can easily create a RabbitMQ cluster by using the RabbitMQ operator by
executing the following command:

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

### Existing RabbitMQ

If you have an existing RabbitMQ that you'd like to use instead of creating a
new one, you must specify how the Knative RabbitMQ Broker can talk to it. For
that, you need to create a secret containing the brokerURL for your existing
cluster:

```sh
R_USERNAME=$(kubectl get secret --namespace default rokn-rabbitmq-admin -o jsonpath="{.data.username}" | base64 --decode)
R_PASSWORD=$(kubectl get secret --namespace default rokn-rabbitmq-admin -o jsonpath="{.data.password}" | base64 --decode)

kubectl create secret generic rokn-rabbitmq-broker-secret \
    --from-literal=brokerURL="amqp://$R_USERNAME:$R_PASSWORD@rokn-rabbitmq-client.default:5672"
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

## Autoscaling (optional)

To get autoscaling (scale to zero as well as up from 0), you can also optionally
install [KEDA based
autoscaler](https://github.com/knative-sandbox/eventing-autoscaler-keda). 

## Demo

### Create a Knative Trigger

Next you need to create a Trigger, specifying which events get routed to
where. For this example, we use a simple PingSource, which generates an event
once a minute.

```
kubectl apply -f - << EOF
  apiVersion: eventing.knative.dev/v1
  kind: Trigger
  metadata:
    name: ping-trigger
    namespace: default
  spec:
    broker: default
    filter:
      attributes:
        type: dev.knative.sources.ping
    subscriber:
      ref:
        apiVersion: serving.knative.dev/v1
        kind: Service
        name: subscriber
EOF
```

Create a Ping Source by executing the following command:

```
kubectl apply -f - << EOF
  apiVersion: sources.knative.dev/v1alpha2
  kind: PingSource
  metadata:
    name: ping-source
  spec:
    schedule: "*/1 * * * *"
    jsonData: '{"message": "Hello world!"}'
    sink:
      ref:
        apiVersion: eventing.knative.dev/v1beta1
        kind: Broker
        name: default
EOF
```

Create an event_display subscriber:

```
kubectl apply -f - << EOF
  apiVersion: serving.knative.dev/v1
  kind: Service
  metadata:
    name: subscriber
    namespace: default
  spec:
    template:
      spec:
        containers:
        - image: gcr.io/knative-releases/knative.dev/eventing-contrib/cmd/event_display
EOF
```

tail the logs on the subscriber's depoyment, and you should see a "Hello world!"
event once per minute; for example using [kail](https://github.com/boz/kail):

```sh
$ kail -d subscriber-4kf8l-deployment
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]: ☁️  cloudevents.Event
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]: Validation: valid
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]: Context Attributes,
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   specversion: 1.0
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   type: dev.knative.sources.ping
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   source: /apis/v1/namespaces/default/pingsources/ping-source
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   id: 1fec78d7-20c2-459f-ac5e-8a797ca7bcdd
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   time: 2020-05-13T17:19:00.000374701Z
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   datacontenttype: application/json
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]: Data,
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   {
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:     "message": "Hello world!"
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   }
```
