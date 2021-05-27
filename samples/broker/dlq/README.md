# RabbitMQ Knative Eventing Broker DLQ Example

## Prerequisites

install the Broker as per [here](../../../broker/README.md)

## Overview

What this demo shows is how the Dead Letter Queue works with the RabbitMQ using
a very simple example. It demonstrates how failed events get sent to Dead Letter
Sink while successfully processed events do not.

## Components

- [failer](../../../cmd/failer/main.go) is a function which takes in a
  CloudEvent and depending on what the specified HTTP response code in the
  message data is will respond with that. So, to simulate a failure, we just
  send it a CloudEvent with a payload of 500 and it's going to simulate a
  failure, by default it will respond with a 200, hence indicating that it
  processed the event successfully and it should be considered handled.

- [pingsource](https://knative.dev/docs/eventing/samples/ping-source/index.html)
  is a Knative source which sends a CloudEvent on pre-defined intervals.

- [event-display](https://github.com/knative/eventing/tree/master/cmd/event_display]
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Broker](../../../broker/README.md)

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
kubectl create ns dlq-demo
```

### Create a RabbitMQ Cluster

Create a RabbitMQ Cluster:

```
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rokn
  namespace: dlq-demo
spec:
  replicas: 1
EOF
```

### Create the RabbitMQ Broker

```Sh
kubectl apply -f - << EOF
  apiVersion: eventing.knative.dev/v1
  kind: Broker
  metadata:
    name: default
    namespace: dlq-demo
    annotations:
      eventing.knative.dev/broker.class: RabbitMQBroker
  spec:
    config:
      apiVersion: rabbitmq.com/v1beta1
      kind: RabbitmqCluster
      name: rokn
    delivery:
      deadLetterSink:
        ref:
          apiVersion: serving.knative.dev/v1
          kind: Service
          name: event-display
          namespace: dlq-demo
      retry: 5
EOF
```

**Note** If you at this point look at the status of the Broker, it's not ready,
because the dead letter Sink is not ready.

```sh
vaikas-a01:wabbit vaikas$ kubectl -n dlq-demo get brokers
NAME      URL                                                        AGE   READY   REASON
default   http://default-broker-ingress.dlq-demo.svc.cluster.local   7s    False   Unable to get the DeadLetterSink's URI
```

### Create the Dead Letter Sink

Then create the Knative Serving Service which will receive any failed events.

```sh
kubectl apply -f - << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: event-display
  namespace: dlq-demo
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
vaikas-a01:wabbit vaikas$ kubectl -n dlq-demo get brokers
NAME      URL                                                        AGE     READY   REASON
default   http://default-broker-ingress.dlq-demo.svc.cluster.local   2m39s   True
```

### Create the Ping Sources

```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1
kind: PingSource
metadata:
  name: ping-source
  namespace: dlq-demo
spec:
  schedule: "*/1 * * * *"
  data: '{"responsecode": 200}'
  sink:
    ref:
      apiVersion: eventing.knative.dev/v1
      kind: Broker
      name: default
      namespace: dlq-demo
EOF
```

```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1
kind: PingSource
metadata:
  name: ping-source-2
  namespace: dlq-demo
spec:
  schedule: "*/1 * * * *"
  data: '{"responsecode": 500}'
  sink:
    ref:
      apiVersion: eventing.knative.dev/v1
      kind: Broker
      name: default
      namespace: dlq-demo
EOF
```

### Create Trigger

```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Trigger
metadata:
  name: failer-trigger
  namespace: dlq-demo
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
      namespace: dlq-demo
EOF
```

### Create Failer

```sh
kubectl apply -f 'https://storage.googleapis.com/knative-nightly/eventing-rabbitmq/latest/failer.yaml'
```

### Check the results

Look at the failer pod logs, you see it's receiving both 200/500 responses.

```sh
vaikas-a01:wabbit vaikas$ kubectl -n dlq-demo -l='serving.knative.dev/service=failer' logs -c user-container
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

However the event-display (the Dead Letter Sink) only sees the failed events
with the response code set to 500.

```sh
vaikas-a01:wabbit vaikas$ kubectl -n dlq-demo -l='serving.knative.dev/service=event-display' logs -c user-container
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: dev.knative.sources.ping
  source: /apis/v1/namespaces/dlq-demo/pingsources/ping-source-2
  id: 166e89ff-19c7-4e9a-a593-9ed30dca0d7d
  time: 2020-10-06T10:35:00.307531386Z
  datacontenttype: application/json
Data,
  {
    "responsecode": 500
  }
```
