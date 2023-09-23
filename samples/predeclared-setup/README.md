# Eventing RabbitMQ Predeclared Setup Example

## Prerequisites and installation

Same as listed [here](../../docs/prerequisites.md)

## Steps

### Create a namespace

Create a new namespace for the demo. All the commands are expected to be
executed from the root of this repo.

```sh
kubectl apply -f samples/predeclared-setup/100-namespace.yaml
```
or
```sh
kubectl create ns predeclared-setup-sample
```

### Create a RabbitMQ Cluster

Create a RabbitMQ Cluster:

```sh
kubectl apply -f samples/predeclared-setup/200-rabbitmq.yaml
```
or
```
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmq
  namespace: predeclared-setup-sample
spec:
  replicas: 1
  override:
    statefulSet:
      spec:
        template:
          spec:
            containers:
            - name: rabbitmq
              env:
              - name: ERL_MAX_PORTS
                value: "4096"
EOF
```

_NOTE_: here we set ERL_MAX_PORTS to prevent unnecessary memory allocation by RabbitMQ and potential memory limit problems. [Read more here](https://github.com/rabbitmq/cluster-operator/issues/959)


After this two steps you are ready to the next steps:
- [Eventing RabbitMQ Broker Setup](#eventing-rabbitmq-broker-setup)
- [Eventing RabbitMQ Source Setup](#eventing-rabbitmq-source-setup)

## Eventing RabbitMQ Broker Setup

Broker does not support predeclared resources at the time.

## Eventing RabbitMQ Source Setup

## Overview

This demo will use a RabbitMQ Source to fetch messages from a Predeclared RabbitMQ Exchange and Queue, convert them into [CloudEvents](https://cloudevents.io/) and send them to a [Sink](https://knative.dev/docs/eventing/sinks/#about-sinks). The complete list of the Source's config parameters are shown [here](../../docs/source/README.md)

## Components

- [RabbitMQ Queue](https://www.rabbitmq.com/tutorials/amqp-concepts.html#queues)

- [RabbitMQ Exchange](https://www.rabbitmq.com/tutorials/amqp-concepts.html#exchanges)

- [RabbitMQ Binding](https://www.rabbitmq.com/tutorials/amqp-concepts.html#bindings)

- [perf-test](https://github.com/rabbitmq/rabbitmq-perf-test) RabbitMQ has a throughput testing tool, PerfTest, that is based on the Java client and can be configured to simulate basic to advanced workloads of messages sent to a RabbitMQ Cluster. More info about the perf-test testing tool can be found [here](../perf-test.help.env.text)

- [event-display](https://github.com/knative/eventing/tree/main/cmd/event_display)
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Source](../../docs/source/README.md)

## Configuration

Demo creates a `RabbitMQ Queue` an `RabbitMQ Exchange` and a `RabbitMQ Binding` between both as the predeclared topology for the `RabbitMQ Source`.

Demo creates a `PerfTest` and has it executes a loop where it sends 1 event per second for 30 seconds, and then no events for 30 seconds to the `RabbitMQ Cluster Exchange` called `eventing-rabbitmq-source`, created by the `RabbitMQ Source`.

Demo creates a `Source` with and exchange configuration for it to read messages from the `eventing-rabbitmq-source` `Exchange` and to send them to the `event-display` `sink` after the translation to CloudEvents.


### Create the Predeclared Topology

#### Using the RabbitMQ Messaging Topology Operator (optional)

If you have it installed in your Cluster just run

```sh
kubectl apply -f samples/predeclared-setup/source-files/100-predeclared-resources.yaml
```

Otherwise just Create a `RabbitMQ Queue` called source-queue, a `RabbitMQ Exchange` called source-exchange and a `RabbitMQ Binding` between both.

### Create the Perf Test Service

This will create a Kubernetes Deployment which sends events to the RabbitMQ Cluster Exchange

```sh
kubectl apply -f samples/predeclared-setup/source-files/200-perf-test.yaml
```

Messages from the `rabbitmq-perf-test` deployment won't reach the RabbitMQ Cluster until the Source is created.

### Create the RabbitMQ Source's Sink

Then create the Knative Serving Service which will receive translated events.

```sh
kubectl apply -f samples/predeclared-setup/300-sink.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: event-display
  namespace: predeclared-setup-sample
spec:
  template:
    spec:
      containers:
      - image: gcr.io/knative-releases/knative.dev/eventing/cmd/event_display
EOF
```

### Create the RabbitMQ Source

```sh
kubectl apply -f samples/predeclared-setup/source-files/300-source.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1alpha1
kind: RabbitmqSource
metadata:
  name: rabbitmq-source
  namespace: predeclared-setup-sample
spec:
  rabbitmqClusterReference:
    name: rabbitmq
    namespace: predeclared-setup-sample
  rabbitmqResourcesConfig:
    # vhost: you-rabbitmq-vhost
    exchangeName: "eventing-rabbitmq-source"
    queueName: "eventing-rabbitmq-source"
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: event-display
      namespace: predeclared-setup-sample
EOF
```

### Check the results

Check the event-display (the Dead Letter Sink) to see if it is receiving events.
It may take a while for the Source to start sending events to the Sink, so be patient :p!

```sh
kubectl -n predeclared-setup-sample -l='serving.knative.dev/service=event-display' logs -c user-container
☁️  cloudevents.Event
Context Attributes,
  specversion: 1.0
  type: dev.knative.rabbitmq.event
  source: /apis/v1/namespaces/predeclared-setup-sample/rabbitmqsources/rabbitmq-source
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
kubectl delete -f samples/predeclared-setup/source-files/300-source.yaml
kubectl delete -f samples/predeclared-setup/
```
