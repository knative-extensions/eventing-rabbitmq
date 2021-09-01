# RabbitMQ Knative Eventing Broker

![RabbitMQ Broker for Knative Eventing](rabbitmq-knative-broker.png)



## Installation

We provide two versions of the RabbitMQ Knative Eventing Broker.

1. Standalone Broker
2. RabbitMQ operator based Broker

### Standalone Broker

This Broker works by utilizing libraries to manage RabbitMQ resources directly and as name implies does not have have dependencies on other operators. Choose this if you do not manage your clusters lifecycle with the [Cluster Operator](https://github.com/rabbitmq/cluster-operator).

[Install Standalone Broker](./standalone.md)

### RabbitMQ operator based Broker

This Broker builds on top of [Cluster Operator](https://github.com/rabbitmq/cluster-operator) and [Messaging Topology Operator](https://github.com/rabbitmq/messaging-topology-operator). As such it requires both of them to be installed. This Broker also will only work with RabbitMQ clusters created and managed by the Cluster Operator and as such if you do not manage your RabbitMQ clusters with it, you must use the [Standalone Broker](./standalone.md).

[Install Operator based Broker](./operator-based.md)

## Autoscaling (optional)

To get autoscaling (scale to zero as well as up from 0), you can also optionally
install
[KEDA based autoscaler](https://github.com/knative-sandbox/eventing-autoscaler-keda).

## Demo

### Create a Broker

Depending on which version of the controller you are running, create a broker either for [standalone](./standalone.md#creating-knative-eventing-broker) or [operator based](./operator-based.md#creating-knative-eventing-broker).

### Create a Knative Trigger

Next you need to create a Trigger, specifying which events get routed to where.
For this example, we use a simple PingSource, which generates an event once a
minute.

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
  apiVersion: sources.knative.dev/v1
  kind: PingSource
  metadata:
    name: ping-source
  spec:
    schedule: "*/1 * * * *"
    data: '{"message": "Hello world!"}'
    sink:
      ref:
        apiVersion: eventing.knative.dev/v1
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
        - image: gcr.io/knative-releases/knative.dev/eventing/cmd/event_display
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
