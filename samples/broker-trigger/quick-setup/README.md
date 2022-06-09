# RabbitMQ Knative Eventing Broker with Trigger and DLQ Example

## Prerequisites and installation

Same as listed [here](../../../docs/operator-based.md#prerequisites)

## Overview

What this demo shows is how the Dead Letter Queue works with the RabbitMQ using
a very simple example. It demonstrates how failed events get sent to Dead Letter
Sink while successfully processed events do not.

## Components

- [failer](../../../cmd/failer/main.go) is a function which takes in a
  CloudEvent and depending on what the specified HTTP response code in the
  message data is will respond with that. So, to simulate a failure, we just
  send it a CloudEvent with a payload of 500, and it's going to simulate a
  failure, by default it will respond with a 200, hence indicating that it
  processed the event successfully, and it should be considered handled.

- [pingsource](https://knative.dev/docs/eventing/samples/ping-source/index.html)
  is a Knative source which sends a CloudEvent on pre-defined intervals.

- [event-display](https://github.com/knative/eventing/tree/main/cmd/event_display)
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Broker](../../../docs/broker.md)

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
kubectl apply -f samples/broker-trigger/quick-setup/100-namespace.yaml
```
or
```sh
kubectl create ns broker-trigger-demo
```

### Create a RabbitMQ Cluster

Create a RabbitMQ Cluster:

```sh
kubectl apply -f samples/broker-trigger/quick-setup/200-rabbitmq.yaml
```
or
```
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmq
  namespace: broker-trigger-demo
spec:
  replicas: 1
EOF
```

### Create the RabbitMQ Broker Config

```sh
kubectl apply -f samples/broker-trigger/quick-setup/250-broker-config.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1alpha1
kind: RabbitmqBrokerConfig
metadata:
  name: default-config
  namespace: broker-trigger-demo
spec:
  rabbitmqClusterReference:
    name: rabbitmq
    namespace: broker-trigger-demo
  queueType: quorum

```

#### Queue Type

Queue type can be configured in the `RabbitmqBrokerConfig`. The supported types are `quorum` (default) and `classic`. Quorum
queues are recommended for fault tolerant multi-node setups. More information on quorum queues can be found [here](https://www.rabbitmq.com/quorum-queues.html). A
minimum RabbitMQ version of (3.8.0) is required to use quorum queues.

Note: Configuring queue type is only supported when using the `RabbitmqBrokerConfig`. If the DEPRECATED `RabbitmqCluster` is used as the broker config, queue type will be `classic`.

### Create the RabbitMQ Broker

```sh
kubectl apply -f samples/broker-trigger/quick-setup/300-broker.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Broker
metadata:
  name: default
  namespace: broker-trigger-demo
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
        namespace: broker-trigger-demo
    retry: 5
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
  namespace: broker-trigger-demo
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
        namespace: broker-trigger-demo
    retry: 5
EOF
```

### Create the Dead Letter Sink

Then create the Knative Serving Service which will receive any failed events.

```sh
kubectl apply -f samples/broker-trigger/quick-setup/400-sink.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: event-display
  namespace: broker-trigger-demo
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
kubectl -n broker-trigger-demo get brokers
NAME      URL                                                                   AGE     READY   REASON
default   http://default-broker-ingress.broker-trigger-demo.svc.cluster.local   2m39s   True
```

### Create the Ping Sources

```sh
kubectl apply -f samples/broker-trigger/quick-setup/500-ping-sources.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1
kind: PingSource
metadata:
  name: ping-source
  namespace: broker-trigger-demo
spec:
  schedule: "*/1 * * * *"
  data: '{"responsecode": 200}'
  sink:
    ref:
      apiVersion: eventing.knative.dev/v1
      kind: Broker
      name: default
      namespace: broker-trigger-demo
EOF
```

```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1
kind: PingSource
metadata:
  name: ping-source-2
  namespace: broker-trigger-demo
spec:
  schedule: "*/1 * * * *"
  data: '{"responsecode": 500}'
  sink:
    ref:
      apiVersion: eventing.knative.dev/v1
      kind: Broker
      name: default
      namespace: broker-trigger-demo
EOF
```

### Create Trigger

```sh
kubectl apply -f samples/broker-trigger/quick-setup/600-trigger.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Trigger
metadata:
  name: failer-trigger
  namespace: broker-trigger-demo
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
      namespace: broker-trigger-demo
EOF
```

### Create Failer

```sh
kubectl apply -f samples/broker-trigger/quick-setup/700-failer.yaml
```
or
```sh
kubectl apply -f 'https://storage.googleapis.com/knative-nightly/eventing-rabbitmq/latest/failer.yaml' -n broker-trigger-demo
```

### Check the results

Look at the failer pod logs, you see it's receiving both 200/500 responses.

```sh
kubectl -n broker-trigger-demo -l='serving.knative.dev/service=failer' logs -c user-container
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
However, the 500 is sent a total of 6 times because we have specified the retry
of 5 (original, plus 5 retries for a total of 6 log entries).

However, the event-display (the Dead Letter Sink) only sees the failed events
with the response code set to 500.

```sh
kubectl -n broker-trigger-demo -l='serving.knative.dev/service=event-display' logs -c user-container
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: dev.knative.sources.ping
  source: /apis/v1/namespaces/broker-trigger-demo/pingsources/ping-source-2
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
kubectl delete ns broker-trigger-demo
```
