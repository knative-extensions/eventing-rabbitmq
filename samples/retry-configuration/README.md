# Eventing RabbitMQ Retries Configuration Example

## Prerequisites and installation

Same as listed [here](../../docs/operator-based.md#prerequisites)

## Overview

What this demo shows is how the Retries configuration works with the RabbitMQ using
a very simple example. It demonstrates how failed events get retried before been sent to Dead Letter
Sink while successfully processed events do not.

## Components

- [failer](../../cmd/failer/main.go) is a function which takes in a
  CloudEvent and depending on what the specified HTTP response code in the
  message data is will respond with that. So, to simulate a failure, we just
  send it a CloudEvent with a payload of 502, and it's going to simulate a
  retryabel failure, by default it will respond with a 200, hence indicating that it
  processed the event successfully, and it should be considered handled.

- [pingsource](https://knative.dev/docs/eventing/samples/ping-source/index.html)
  is a Knative source which sends a CloudEvent on pre-defined intervals.

- [event-display](https://github.com/knative/eventing/tree/main/cmd/event_display)
  which is a tool that logs the CloudEvent that it receives formatted nicely.

- [RabbitMQ Broker](../../docs/broker.md)

## Configuration

Demo creates two `PingSource`s and has them send an event once a minute. One of
them sends an event that has responsecode set to 200 (event processed
successfully) and one that has responsecode set to 502 (event processing
failed). The status 502 is one of the default retryable status for the cehttp client used by both `Triggers` and `Sources`.

Demo creates a `Broker` with the delivery configuration that specifies that
failed events will be delivered to `event-display`.

Demo creates a `Trigger` that wires `PingSource` events to go to the `failer`.

## Steps

### Create a namespace

Create a new namespace for the demo. All the commands are expected to be
executed from the root of this repo.

```sh
kubectl apply -f samples/retry-configuration/100-namespace.yaml
```
or
```sh
kubectl create ns retry-config-sample
```

### Create a RabbitMQ Cluster

Create a RabbitMQ Cluster:

```sh
kubectl apply -f samples/retry-configuration/200-rabbitmq.yaml
```
or
```
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmq
  namespace: retry-config-sample
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
kubectl apply -f samples/retry-configuration/broker-files/100-broker-config.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1alpha1
kind: RabbitmqBrokerConfig
metadata:
  name: default-config
  namespace: retry-config-sample
spec:
  rabbitmqClusterReference:
    name: rabbitmq
    namespace: retry-config-sample
  queueType: quorum
EOF
```

### Create the RabbitMQ Broker

```sh
kubectl apply -f samples/retry-configuration/broker-files/200-broker.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Broker
metadata:
  name: default
  namespace: retry-config-sample
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
        namespace: retry-config-sample
    retry: 5
    backoffPolicy: "linear"
    backoffDelay: "PT1S"
EOF
```

### Create the Dead Letter Sink

Then create the Knative Serving Service which will receive any failed events.

```sh
kubectl apply -f samples/retry-configuration/300-sink.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: event-display
  namespace: retry-config-sample
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
kubectl -n retry-config-sample get brokers
NAME      URL                                                                   AGE     READY   REASON
default   http://default-broker-ingress.retry-config-sample.svc.cluster.local   2m39s   True
```

### Create the Ping Sources

```sh
kubectl apply -f samples/retry-configuration/broker-files/300-ping-sources.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1
kind: PingSource
metadata:
  name: ping-source
  namespace: retry-config-sample
spec:
  schedule: "*/1 * * * *"
  data: '{"responsecode": 200}'
  sink:
    ref:
      apiVersion: eventing.knative.dev/v1
      kind: Broker
      name: default
      namespace: retry-config-sample
EOF
```

```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1
kind: PingSource
metadata:
  name: ping-source-2
  namespace: retry-config-sample
spec:
  schedule: "*/1 * * * *"
  data: '{"responsecode": 502}'
  sink:
    ref:
      apiVersion: eventing.knative.dev/v1
      kind: Broker
      name: default
      namespace: retry-config-sample
EOF
```

### Create Trigger

```sh
kubectl apply -f samples/retry-configuration/broker-files/400-trigger.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Trigger
metadata:
  name: failer-trigger
  namespace: retry-config-sample
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
      namespace: retry-config-sample
EOF
```

### Create Failer

```sh
ko apply -f samples/retry-configuration/broker-files/500-failer.yaml
```

### Check the results

Look at the failer pod logs, you see it's receiving both 200/500 responses.

```sh
kubectl -n retry-config-sample -l='serving.knative.dev/service=failer' logs -c user-container
2020/10/06 10:35:00 using response code: 200
2020/10/06 10:35:00 using response code: 502
2020/10/06 10:35:00 using response code: 502
2020/10/06 10:35:00 using response code: 502
2020/10/06 10:35:01 using response code: 502
2020/10/06 10:35:01 using response code: 502
```

You see there are both 200 / 502 events there. And more importantly, you can see
that 200 is only sent once to the failer since it's processed correctly.
However, the 500 is sent a total of 5 times because we have specified the retry
of 4 (original, plus 4 retries for a total of 5 log entries), each time with a backoff time of `200ms`*retry number, since it is a linear backoff strategy.

However, the event-display (the Dead Letter Sink) only sees the failed events
with the response code set to 500.

```sh
kubectl -n retry-config-sample -l='serving.knative.dev/service=event-display' logs -c user-container
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: dev.knative.sources.ping
  source: /apis/v1/namespaces/retry-config-sample/pingsources/ping-source-2
  id: 166e89ff-19c7-4e9a-a593-9ed30dca0d7d
  time: 2020-10-06T10:35:00.307531386Z
  datacontenttype: application/json
Data,
  {
    "responsecode": 502
  }
```

### Cleanup

```sh
kubectl delete ns retry-config-sample
```

## Eventing RabbitMQ Source Setup

## Overview

This demo will use a RabbitMQ Source to fetch messages from a RabbitMQ Exchange, convert them into [CloudEvents](https://cloudevents.io/) and send them to a [Sink](https://knative.dev/docs/eventing/sinks/#about-sinks) that answers with an error status code, for the Source to retry the CloudEvent requests. The complete list of the Source's config parameters are shown [here](../../docs/source.md)

## Components

- [perf-test](https://github.com/rabbitmq/rabbitmq-perf-test) RabbitMQ has a throughput testing tool, PerfTest, that is based on the Java client and can be configured to simulate basic to advanced workloads of messages sent to a RabbitMQ Cluster. More info about the perf-test testing tool can be found [here](../perf-test.help.env.text)

- [failer](../../cmd/failer/main.go) is a function which takes in a
  CloudEvent and depending on what the specified HTTP response code in the
  message data is will respond with that. So, to simulate a failure, we just
  set the default status code response to 502, hence indicating that the
  event processing had an error, and the request should retry.

- [RabbitMQ Source](../../docs/source.md)

## Configuration

Demo creates a `PerfTest` and has it executes a loop where it sends 1 event per minute to the `RabbitMQ Cluster Exchange` called `eventing-rabbitmq-source`, created by the `RabbitMQ Source`.

Demo creates a `Source` with and exchange configuration for it to read messages from the `eventing-rabbitmq-source` `Exchange` and to send them to the `failer` `sink` after the translation to CloudEvents, and retry when the `failer` responds with an error `statusCode`.

### Create the Perf Test Service

This will create a Kubernetes Deployment which sends events to the RabbitMQ Cluster Exchange

```sh
kubectl apply -f samples/retry-configuration/source-files/100-perf-test.yaml
```

Messages from the `rabbitmq-perf-test` deployment won't reach the RabbitMQ Cluster until the Source is created, which results in the creation of the Exchange and Queue where the messages are going to be sent

### Create Failer

The Failer will be used as the `Source` `Sink`

```sh
ko apply -f samples/retry-configuration/broker-files/500-failer.yaml
```

### Create the RabbitMQ Source

```sh
kubectl apply -f samples/retry-configuration/source-files/200-source.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1alpha1
kind: RabbitmqSource
metadata:
  name: rabbitmq-source
  namespace: retry-config-sample
spec:
  rabbitmqClusterReference:
    name: rabbitmq
    namespace: retry-config-sample
  rabbitmqResourcesConfig:
    exchangeName: "eventing-rabbitmq-source"
    queueName: "eventing-rabbitmq-source"
  delivery:
    retry: 4
    backoffPolicy: "linear"
    backoffDelay: "PT0.2S"
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: failer
      namespace: retry-config-sample
EOF
```

### Check the results

Look at the failer pod logs, you see it's receiving both 200/500 responses.

```sh
kubectl -n retry-config-sample -l='serving.knative.dev/service=failer' logs -c user-container
2020/10/06 10:35:00 using response code: 502
2020/10/06 10:35:00 using response code: 502
2020/10/06 10:35:00 using response code: 502
2020/10/06 10:35:01 using response code: 502
2020/10/06 10:35:01 using response code: 502
```

You see there are 502 events there. And more importantly, you can see that the 500 is sent a total of 5 times because we have specified the retry of 4 (original, plus 4 retries for a total of 5 log entries), each time with a backoff time of `200ms`*retry number, since it is a linear backoff strategy.

### Cleanup

```sh
kubectl delete -f samples/retry-configuration/source-files/200-source.yaml
kubectl delete -f samples/retry-configuration/
```
