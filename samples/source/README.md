# RabbitMQ Knative Eventing Source Example

## Prerequisites

install the Source as per [here](../../source/DEVELOPMENT.md)

## Overview

What this demo shows is how the RabbitMQ Source translates messages in any format (in this case json) sent to a RabbitMQ Exchange into [CloudEvents](https://cloudevents.io/), showing the basic topology that it useas and how it handles different workloads. It's recommended for the dev to play with the Source's config parameters, shown [here](../../source/README.md)

## Components

- [perf-test](https://github.com/rabbitmq/rabbitmq-perf-test) RabbitMQ has a throughput testing tool, PerfTest, that is based on the Java client and can be configured to simulate from basic to advanced workloads of messages flowing to a RabbitMQ Cluster.

- [event-display](https://github.com/knative/eventing/tree/master/cmd/event_display]
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Source](../../source/README.md)

## Configuration

Demo creates a `PerfTest` and has it send 1 event for 30 seconds, and then 0 events for 30 seconds to the `RabbitMQ Cluster` `eventing-rabbitmq-source` `Exchange`, this in a loop.

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

### Create the Perf Test Service

This will send events to the RabbitMQ Cluster Exchange

```sh
kubectl apply -f samples/source/300-perf-test.yaml
```

Right now, the events are not been sent to the Exchange cause the Source is not created, and is the Source the one that creates the Exchange, Channel and Queues needed for the message Translation to CloudEvents

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
  brokers: "rabbitmq:5672/"
  topic: ""
  user:
    secretKeyRef:
      name: rabbitmq-default-user
      key: username
  password:
    secretKeyRef:
      name: rabbitmq-default-user
      key: password
  channel_config:
    global_qos: false
  exchange_config:
    name: "eventing-rabbitmq-source"
    type: "fanout"
    durable: false
    auto_deleted: false
    internal: false
    nowait: false
  queue_config:
    name: "eventing-rabbitmq-source"
    routing_key: ""
    durable: false
    delete_when_unused: true
    exclusive: true
    nowait: false
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
