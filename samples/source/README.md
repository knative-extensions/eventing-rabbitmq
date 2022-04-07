# RabbitMQ Knative Eventing Source Example

## Prerequisites

Same as listed [here](../../docs/source.md#prerequisites)

## Overview

This demo will use a RabbitMQ Source to fetch messages from a RabbitMQ Exchange, convert them into [CloudEvents](https://cloudevents.io/) and send them to a [Sink](https://knative.dev/docs/eventing/sinks/#about-sinks). The complete list of the Source's config parameters are shown [here](../../docs/source.md)

## Components

- [perf-test](https://github.com/rabbitmq/rabbitmq-perf-test) RabbitMQ has a throughput testing tool, PerfTest, that is based on the Java client and can be configured to simulate basic to advanced workloads of messages sent to a RabbitMQ Cluster.

- [event-display](https://github.com/knative/eventing/tree/main/cmd/event_display)
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Source](../../docs/source.md)

## Configuration

Demo creates a `PerfTest` and has it executes a loop where it sends 1 event per second for 30 seconds, and then no events for 30 seconds to the `RabbitMQ Cluster Exchange` called `eventing-rabbitmq-source`, created by the `RabbitMQ Source`.

Demo creates a `Source` with and exchange configuration for it to read messages from the `eventing-rabbitmq-source` `Exchange` and to send them to the `event-display` `sink` after the translation to CloudEvents.

## Steps

### Create a namespace

Create a new namespace for the demo. All the commands are expected to be
executed from the root of this repo.

```sh
kubectl apply -f samples/source/100-namespace.yaml
```
or
```sh
kubectl create ns source-demo
```

### Create a RabbitMQ Cluster

Create a RabbitMQ Cluster:

```sh
kubectl apply -f samples/source/200-rabbitmq.yaml
```
or
```
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmq
  namespace: source-demo
spec:
  replicas: 1
EOF
```

### Add RabbitMQ http uri to secret

After creating the RabbitMQ cluster in previous step, add the RabbitMQ http uri to its default user secret.
The RabbitMQ default user credentials are stored in a Kubernetes secret called 'NAME-default-user', where NAME is the name of the RabbitmqCluster object.
In this example, the secret name is 'rabbitmq-default-user' in namespace 'source-demo'.

To edit the RabbitMQ default user secret:
```sh
kubectl -n source-demo edit secret rabbitmq-default-user
```

Add uri to the Secret `stringData`
```yaml
kind: Secret
metadata:
  name: rabbitmq-default-user
  namespace: source-demo
type: Opaque
data:
  ...
stringData:
  uri: rabbitmq.source-demo:15672 # 'name-of-RabbitmqCluster.the-namespace:http-port'
```

### Create the Perf Test Service

This will create a Kubernetes Deployment which sends events to the RabbitMQ Cluster Exchange

```sh
kubectl apply -f samples/source/300-perf-test.yaml
```

Messages from the `rabbitmq-perf-test` deployment won't reach the RabbitMQ Cluster until the Source is created, which results in the creation of the Exchange and Queue where the messages are going to be sent

### Create the RabbitMQ Source's Sink

Then create the Knative Serving Service which will receive translated events.

```sh
kubectl apply -f samples/source/400-sink.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: event-display
  namespace: source-demo
spec:
  template:
    spec:
      containers:
      - image: gcr.io/knative-releases/knative.dev/eventing/cmd/event_display
EOF
```

### Create the RabbitMQ Source

```sh
kubectl apply -f samples/source/500-source.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1alpha1
kind: RabbitmqSource
metadata:
  name: rabbitmq-source
  namespace: source-demo
spec:
  broker: "rabbitmq:5672/"
  user:
    secretKeyRef:
      name: rabbitmq-default-user
      key: username
  password:
    secretKeyRef:
      name: rabbitmq-default-user
      key: password
  connectionSecret:
    name: rabbitmq-default-user
  channelConfig:
    globalQos: false
  exchangeConfig:
    name: "eventing-rabbitmq-source"
    type: "fanout"
    durable: false
    autoDelete: false
  queueConfig:
    name: "eventing-rabbitmq-source"
    routingKey: ""
    durable: false
    autoDelete: true
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: rabbitmq-source-sink
      namespace: source-demo
EOF
```

### Check the results

Check the rabbitmq-source-sink (the Dead Letter Sink) to see if it is receiving events.
It may take a while for the Source to start sending events to the Sink, so be patient :p!

```sh
kubectl -n source-demo -l='serving.knative.dev/service=rabbitmq-source-sink' logs -c user-container
☁️  cloudevents.Event
Context Attributes,
  specversion: 1.0
  type: dev.knative.rabbitmq.event
  source: /apis/v1/namespaces/source-demo/rabbitmqsources/rabbitmq-source
  subject: f147099d-c64d-41f7-b8eb-a2e53b228349
  id: f147099d-c64d-41f7-b8eb-a2e53b228349
  time: 2021-12-16T20:11:39.052276498Z
  datacontenttype: application/json
Data,
  {
    ...
    Random Data
    ...
  }
```

### Cleanup

```sh
kubectl delete ns source-demo
```
