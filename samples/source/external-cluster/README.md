# RabbitMQ Knative Eventing Source Example

## Prerequisites

- Same as listed [here](../../../docs/source.md#prerequisites)
- A working RabbitMQ external instance exposed via an accessible IP/URL, an example on how to setup this can be found [here](./resources/rabbitmq.yml)
- An RabbitMQ `Exchange` called `eventing-rabbitmq-source`, `Queue` called `eventing-rabbitmq-source` and `Binding` between both already declared, since the example used the `predeclared` flag

Note: An external RabbitMQ instance can be used, but if you want to use the `Source` without predeclared resources (specifically the `Exchange`, `Binding` and `Queue`), the `RabbitMQ Message Topology Operator` must be installed in the same Kubernetes Cluster as the `Source`.

## Overview

This demo will use a RabbitMQ Source to fetch messages from a RabbitMQ Exchange, convert them into [CloudEvents](https://cloudevents.io/) and send them to a [Sink](https://knative.dev/docs/eventing/sinks/#about-sinks). The complete list of the Source's config parameters are shown [here](../../../docs/source.md)

## Components

- [perf-test](https://github.com/rabbitmq/rabbitmq-perf-test) RabbitMQ has a throughput testing tool, PerfTest, that is based on the Java client and can be configured to simulate basic to advanced workloads of messages sent to a RabbitMQ Cluster. More info about the perf-test testing tool can be found [here](../perf-test.help.env.text)

- [event-display](https://github.com/knative/eventing/tree/main/cmd/event_display)
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Source](../../../docs/source.md)

## Configuration

Demo creates a `PerfTest` and has it executes a loop where it sends 1 event per second for 30 seconds, and then no events for 30 seconds to the `RabbitMQ Cluster Exchange` called `eventing-rabbitmq-source` predeclared by the user.

Demo creates a `Source` to read messages from the `eventing-rabbitmq-source` `Exchange` and to send them to the `event-display` `sink` after the translation to CloudEvents.

## Steps

### Create a namespace

Create a new namespace for the demo. All the commands are expected to be
executed from the root of this repo.

```sh
kubectl apply -f samples/source/external-cluster/100-namespace.yaml
```
or
```sh
kubectl create ns source-demo
```

### Create a Kubernetes Secret to connect to the External Cluster

This secret must contain the `username`, `password` and `uri` keys, this `uri` must be the URL to the RabbitMQ's Management UI.
In this example, the secret name is 'rabbitmq-secret-credentials' in namespace 'source-demo'.

```sh
kubectl apply -f samples/source/external-cluster/200-secret.yaml
```

### Create the Perf Test Service

This will create a Kubernetes Deployment which sends events to the RabbitMQ Cluster Exchange

```sh
kubectl apply -f samples/source/external-cluster/300-perf-test.yaml
```

Messages from the `rabbitmq-perf-test`

### Create the RabbitMQ Source's Sink

Then create the Knative Serving Service which will receive translated events.

```sh
kubectl apply -f samples/source/external-cluster/400-sink.yaml
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
kubectl apply -f samples/source/external-cluster/500-source.yaml
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
  rabbitmqClusterReference:
    namespace: source-demo
    connectionSecret: rabbitmq-secret-credentials
  rabbitmqResourcesConfig:
    predeclared: true
    exchangeName: "eventing-rabbitmq-source"
    queueName: "eventing-rabbitmq-source"
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: rabbitmq-source-sink
      namespace: source-demo
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
kubectl delete -f samples/source/external-cluster/500-source.yaml
kubectl delete -f samples/source/external-cluster/
```
