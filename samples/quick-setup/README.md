# Eventing RabbitMQ Quick Setup Example

## Prerequisites and installation

Same as listed [here](../../docs/prerequisites.md)

## Overview

What this demo shows is how the Dead Letter Queue works with the RabbitMQ using
a very simple example. It demonstrates how failed events get sent to Dead Letter
Sink while successfully processed events do not.

## Components

- [failer](../../cmd/failer/main.go) is a function which takes in a
  CloudEvent and depending on what the specified HTTP response code in the
  message data is will respond with that. So, to simulate a failure, we just
  send it a CloudEvent with a payload of 500, and it's going to simulate a
  failure, by default it will respond with a 200, hence indicating that it
  processed the event successfully, and it should be considered handled.

- [pingsource](https://knative.dev/docs/eventing/samples/ping-source/index.html)
  is a Knative source which sends a CloudEvent on pre-defined intervals.

- [event-display](https://github.com/knative/eventing/tree/main/cmd/event_display)
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Broker](../../docs/broker/README.md)

## Configuration

Demo creates two `PingSource`s and has them send an event once a minute. One of
them sends an event that has responsecode set to 200 (event processed
successfully) and one that has responsecode set to 500 (event processing
failed).

Demo creates a `Broker` with the delivery configuration that specifies that
failed events will be delivered to `event-display`.

Demo creates a `Trigger` that wires `PingSource` events to go to the `failer`.

## Steps

### Create a namespace

Create a new namespace for the demo. All the commands are expected to be
executed from the root of this repo.

```sh
kubectl apply -f samples/quick-setup/100-namespace.yaml
```
or
```sh
kubectl create ns quick-setup-sample
```

### Create a RabbitMQ Cluster

Create a RabbitMQ Cluster:

```sh
kubectl apply -f samples/quick-setup/200-rabbitmq.yaml
```
or
```
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmq
  namespace: quick-setup-sample
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

### Create the RabbitMQ Broker Config

```sh
kubectl apply -f samples/quick-setup/broker-files/100-broker-config.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1alpha1
kind: RabbitmqBrokerConfig
metadata:
  name: default-config
  namespace: quick-setup-sample
spec:
  rabbitmqClusterReference:
    name: rabbitmq
    namespace: quick-setup-sample
  queueType: quorum
EOF
```

#### Queue Type

Queue type can be configured in the `RabbitmqBrokerConfig`. The supported types are `quorum` (default) and `classic`. Quorum
queues are recommended for fault tolerant multi-node setups. More information on quorum queues can be found [here](https://www.rabbitmq.com/quorum-queues.html). A
minimum RabbitMQ version of (3.8.0) is required to use quorum queues.

Note: Configuring queue type is only supported when using the `RabbitmqBrokerConfig`. If the DEPRECATED `RabbitmqCluster` is used as the broker config, queue type will be `classic`.

### Create the RabbitMQ Broker

```sh
kubectl apply -f samples/quick-setup/broker-files/200-broker.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Broker
metadata:
  name: default
  namespace: quick-setup-sample
  annotations:
    eventing.knative.dev/broker.class: RabbitMQBroker
spec:
  config:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: RabbitmqBrokerConfig
    name: default-config
  delivery:
    deadLetterSink:
      ref:
        apiVersion: serving.knative.dev/v1
        kind: Service
        name: event-display
        namespace: quick-setup-sample
EOF
```

### [DEPRECATED] Create the RabbitMQ Broker (with RabbitmqCluster as config)

While using a reference to a RabbitmqCluster directly is supported, it's deprecated and will be removed in a future release

```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Broker
metadata:
  name: default
  namespace: quick-setup-sample
  annotations:
    eventing.knative.dev/broker.class: RabbitMQBroker
spec:
  config:
    apiVersion: rabbitmq.com/v1beta1
    kind: RabbitmqCluster
    name: rabbitmq
  delivery:
    deadLetterSink:
      ref:
        apiVersion: serving.knative.dev/v1
        kind: Service
        name: event-display
        namespace: quick-setup-sample
EOF
```

### Create the Dead Letter Sink

Then create the Knative Serving Service which will receive any failed events.

```sh
kubectl apply -f samples/quick-setup/300-sink.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: event-display
  namespace: quick-setup-sample
spec:
  template:
    spec:
      containers:
      - image: gcr.io/knative-releases/knative.dev/eventing/cmd/event_display
EOF
```

Now the Broker will become ready, might take a few seconds to get the Service up
and running.

```sh
kubectl -n quick-setup-sample get brokers
NAME      URL                                                                   AGE     READY   REASON
default   http://default-broker-ingress.quick-setup-sample.svc.cluster.local   2m39s   True
```

### Create the Ping Sources

```sh
kubectl apply -f samples/quick-setup/broker-files/300-ping-sources.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1
kind: PingSource
metadata:
  name: ping-source
  namespace: quick-setup-sample
spec:
  schedule: "*/1 * * * *"
  data: '{"responsecode": 200}'
  sink:
    ref:
      apiVersion: eventing.knative.dev/v1
      kind: Broker
      name: default
      namespace: quick-setup-sample
EOF
```

```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1
kind: PingSource
metadata:
  name: ping-source-2
  namespace: quick-setup-sample
spec:
  schedule: "*/1 * * * *"
  data: '{"responsecode": 500}'
  sink:
    ref:
      apiVersion: eventing.knative.dev/v1
      kind: Broker
      name: default
      namespace: quick-setup-sample
EOF
```

### Create Trigger

```sh
kubectl apply -f samples/quick-setup/broker-files/400-trigger.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Trigger
metadata:
  name: failer-trigger
  namespace: quick-setup-sample
  annotations:
    # Value must be between 1 and 1000
    # A value of 1 RabbitMQ Trigger behaves as a FIFO queue
    # Values above 1 break message ordering guarantees and can be seen as more performance oriented.
    # rabbitmq.eventing.knative.dev/parallelism: "10"
spec:
  broker: default
  filter:
    attributes:
      type: dev.knative.sources.ping
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: failer
      namespace: quick-setup-sample
EOF
```

### Create Failer

```sh
kubectl apply -f samples/quick-setup/broker-files/500-failer.yaml
```
or
```sh
kubectl apply -f 'https://storage.googleapis.com/knative-nightly/eventing-rabbitmq/latest/failer.yaml' -n quick-setup-sample
```

### Check the results

Look at the failer pod logs, you see it's receiving both 200/500 responses.

```sh
kubectl -n quick-setup-sample -l='serving.knative.dev/service=failer' logs -c user-container
2020/10/06 10:35:00 using response code: 200
2020/10/06 10:35:00 using response code: 500
2020/10/06 10:35:00 using response code: 500
2020/10/06 10:35:00 using response code: 500
2020/10/06 10:35:01 using response code: 500
2020/10/06 10:35:01 using response code: 500
2020/10/06 10:35:03 using response code: 500
```

You see there are both 200 / 500 events there. And more importantly, you can see
that 200 is only sent once to the failer since it's processed correctly.
However, the 500 is sent a total of 6 times because the default retry for the `Trigger` is 5
(original, plus 5 retries for a total of 6 log entries).

However, the event-display (the Dead Letter Sink) only sees the failed events
with the response code set to 500.

```sh
kubectl -n quick-setup-sample -l='serving.knative.dev/service=event-display' logs -c user-container
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: dev.knative.sources.ping
  source: /apis/v1/namespaces/quick-setup-sample/pingsources/ping-source-2
  id: 166e89ff-19c7-4e9a-a593-9ed30dca0d7d
  time: 2020-10-06T10:35:00.307531386Z
  datacontenttype: application/json
Data,
  {
    "responsecode": 500
  }
```

### Cleanup

```sh
kubectl delete ns quick-setup-sample
```

## Eventing RabbitMQ Source Setup

## Overview

This demo will use a RabbitMQ Source to fetch messages from a RabbitMQ Exchange, convert them into [CloudEvents](https://cloudevents.io/) and send them to a [Sink](https://knative.dev/docs/eventing/sinks/#about-sinks). The complete list of the Source's config parameters are shown [here](../../docs/source/README.md)

## Components

- [perf-test](https://github.com/rabbitmq/rabbitmq-perf-test) RabbitMQ has a throughput testing tool, PerfTest, that is based on the Java client and can be configured to simulate basic to advanced workloads of messages sent to a RabbitMQ Cluster. More info about the perf-test testing tool can be found [here](../perf-test.help.env.text)

- [event-display](https://github.com/knative/eventing/tree/main/cmd/event_display)
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Source](../../docs/source/README.md)

## Configuration

Demo creates a `PerfTest` and has it executes a loop where it sends 1 event per second for 30 seconds, and then no events for 30 seconds to the `RabbitMQ Cluster Exchange` called `eventing-rabbitmq-source`, created by the `RabbitMQ Source`.

Demo creates a `Source` with and exchange configuration for it to read messages from the `eventing-rabbitmq-source` `Exchange` and to send them to the `event-display` `sink` after the translation to CloudEvents.

### Create the Perf Test Service

This will create a Kubernetes Deployment which sends events to the RabbitMQ Cluster Exchange

```sh
kubectl apply -f samples/quick-setup/source-files/100-perf-test.yaml
```

Messages from the `rabbitmq-perf-test` deployment won't reach the RabbitMQ Cluster until the Source is created, which results in the creation of the Exchange and Queue where the messages are going to be sent

### Create the RabbitMQ Source's Sink

Then create the Knative Serving Service which will receive translated events.

```sh
kubectl apply -f samples/quick-setup/300-sink.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: event-display
  namespace: quick-setup-sample
spec:
  template:
    spec:
      containers:
      - image: gcr.io/knative-releases/knative.dev/eventing/cmd/event_display
EOF
```

### Create the RabbitMQ Source

```sh
kubectl apply -f samples/quick-setup/source-files/200-source.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1alpha1
kind: RabbitmqSource
metadata:
  name: rabbitmq-source
  namespace: quick-setup-sample
spec:
  rabbitmqClusterReference:
    name: rabbitmq
    namespace: quick-setup-sample
  rabbitmqResourcesConfig:
    exchangeName: "eventing-rabbitmq-source"
    queueName: "eventing-rabbitmq-source"
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: event-display
      namespace: quick-setup-sample
EOF
```

### Check the results

Check the event-display (the Dead Letter Sink) to see if it is receiving events.
It may take a while for the Source to start sending events to the Sink, so be patient :p!

```sh
kubectl -n quick-setup-sample -l='serving.knative.dev/service=event-display' logs -c user-container
☁️  cloudevents.Event
Context Attributes,
  specversion: 1.0
  type: dev.knative.rabbitmq.event
  source: /apis/v1/namespaces/quick-setup-sample/rabbitmqsources/rabbitmq-source
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
kubectl delete -f samples/quick-setup/source-files/200-source.yaml
kubectl delete -f samples/quick-setup/
```
