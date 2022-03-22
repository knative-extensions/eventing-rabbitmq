# Trigger example with customizations

## Prerequisites and installation

- Follow instructions listed [here](../../broker/operator-based.md#prerequisites)
- Install [KO](https://github.com/google/ko). It is used to create the images that will be used in this demo
- Set `KO_DOCKER_REPO` to an accessible container registry

## Overview

This doc and demos highlight the various customizations that can be done on a Trigger along with their usecases and implications.

### Trigger Pre-Fetch Count
Trigger has a configurable annotation `rabbitmq.eventing.knative.dev/prefetchCount`. The following are effects of setting this parameter to `n`:

- Prefetch count is set on the RabbitMQ channel and queue created for this trigger. The channel will receive a maximum of `n` number of messages at once.
- The trigger will create `n` workers to consume messages off the queue and dispatch to the sink.

If this value is unset, it will default to `1`. This means the trigger will only handle one event at a time. This will preserve the order of the messages in a queue but
will make the trigger a bottleneck. A slow processing sink will result in low overall throughput. Setting a value higher than 1 will result in `n` events being handled at
a time by the trigger but ordering won't be guaranteed as events are sent to the sink.

The following demo highlights the benefits and tradeoffs of setting the prefetch count to > 1 and leaving it as 1.

#### Configuration
- Create a RabbitMQ cluster and broker
- Create a source that will send 10 events all at once to a broker
- Create a first-in-first-out (FIFO) trigger with prefetch count unset (defaults to 1)
- Create a high-throughput trigger with prefetch count set to 10
- Create a slow sink for each trigger
- Observe the results through the logs of the sinks

#### Components
New components introduced in this demo:

- [event-sender](./event-sender)
Is used to send `n` messages to a broker.

- [event-display](./event-display)
Is used as a "slow" sink. It will wait for `delay` number of seconds for each event it consumes.

#### Steps
Execute the following steps/commands from the root of this repo.

#### Create a namespace
Create a new namespace for the demo. This will make cleanup easier.

```sh
kubectl apply -f samples/trigger-customizations/100-namespace.yaml
```
or
```sh
kubectl create ns trigger-demo
```

#### Create a RabbitMQ Cluster

```sh
kubectl apply -f samples/trigger-customizations/200-rabbitmq.yaml
```
or
```
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmq
  namespace: trigger-demo
spec:
  replicas: 1
EOF
```

#### Create a Broker
```sh
kubectl apply -f samples/trigger-customizations/300-broker.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Broker
metadata:
  name: default
  namespace: trigger-demo
  annotations:
    eventing.knative.dev/broker.class: RabbitMQBroker
spec:
  config:
    apiVersion: rabbitmq.com/v1beta1
    kind: RabbitmqCluster
    name: rabbitmq
EOF
```

#### Create the 2 Triggers

```sh
kubectl apply -f samples/trigger-customizations/400-trigger.yaml
```
or
```sh
kubectl apply -f - << EOF
apiVersion: eventing.knative.dev/v1
kind: Trigger
metadata:
  name: fifo-trigger
  namespace: trigger-demo
spec:
  broker: default
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: fifo-event-display
      namespace: trigger-demo
---
apiVersion: eventing.knative.dev/v1
kind: Trigger
metadata:
  name: high-throughput-trigger
  namespace: trigger-demo
  annotations:
    # Value must be between 1 and 1000
    # A value of 1 RabbitMQ Trigger behaves as a FIFO queue
    # Values above 1 break message ordering guarantees but can be seen as more performance oriented.
    rabbitmq.eventing.knative.dev/prefetchCount: "10"
spec:
  broker: default
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: high-throughput-event-display
      namespace: trigger-demo
EOF
```

#### Create the 2 Sinks
NOTE: ko is used here to create the custom images. Ensure `KO_DOCKER_REPO` is set to an accessible repository for the images.
```sh
ko apply -f samples/trigger-customizations/500-sink.yaml
```
or
```sh
ko apply -f - << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: fifo-event-display
  namespace: trigger-demo
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/min-scale: "1"
    spec:
      containers:
      - image: ko://knative.dev/eventing-rabbitmq/samples/trigger-customizations/event-display
        args:
          - --delay=20
---
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: high-throughput-event-display
  namespace: trigger-demo
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/min-scale: "1"
    spec:
      containers:
        - image: ko://knative.dev/eventing-rabbitmq/samples/trigger-customizations/event-display
          args:
            - --delay=20
EOF
```

#### Wait for resources to become ready
Before sending messages, wait for all relevant resources to become Ready
```sh
kubectl get broker -n trigger-demo
NAME      URL                                                            AGE   READY   REASON
default   http://default-broker-ingress.trigger-demo.svc.cluster.local   95s   True

kubectl get trigger -n trigger-demo
NAME                      BROKER    SUBSCRIBER_URI                                                        AGE    READY   REASON
fifo-trigger              default   http://fifo-event-display.trigger-demo.svc.cluster.local              5m1s   True
high-throughput-trigger   default   http://high-throughput-event-display.trigger-demo.svc.cluster.local   5m1s   True

kubectl get services.serving.knative.dev -n trigger-demo
NAME                            URL                                                             LATESTCREATED                         LATESTREADY                           READY   REASON
fifo-event-display              http://fifo-event-display.trigger-demo.example.com              fifo-event-display-00001              fifo-event-display-00001              True
high-throughput-event-display   http://high-throughput-event-display.trigger-demo.example.com   high-throughput-event-display-00001   high-throughput-event-display-00001   True
```

#### Create the Source
Create the source to start sending messages.
<br/>
NOTE: ensure the `--sink` is set to the broker's ingress URL.

```sh
ko apply -f samples/trigger-customizations/600-event-sender.yaml
```
or
```sh
ko apply -f - << EOF
apiVersion: v1
kind: Pod
metadata:
  name: event-sender
  namespace: trigger-demo
spec:
  restartPolicy: Never
  containers:
    - name: event-sender
      image: ko://knative.dev/eventing-rabbitmq/samples/trigger-customizations/event-sender
      args:
        - --sink=http://default-broker-ingress.trigger-demo.svc.cluster.local
        - --event={"specversion":"1.0","type":"SenderEvent","source":"Sender","datacontenttype":"application/json","data":{}}
        - --num-messages=10
EOF
```

### Start watching the results
Once the event-display pods are up and running, watch for logs in the FIFO event display. Once the sender has been created (and after the delay), you should see
events being consumed in order. But this sink will take longer to process all of the incoming events.

```sh
kubectl -n trigger-demo -l='serving.knative.dev/service=fifo-event-display' logs -c user-container -f
processed event with ID: 1
processed event with ID: 2
processed event with ID: 3
processed event with ID: 4
processed event with ID: 5
processed event with ID: 6
processed event with ID: 7
processed event with ID: 8
processed event with ID: 9
processed event with ID: 10
```

And in a different shell, watch for logs from the high-throughput sink. Once the sender is created (and after the delay), you should
see all the events getting consumed fairly quickly. But they will likely be out of order.

```sh
kubectl -n trigger-demo -l='serving.knative.dev/service=high-throughput-event-display' logs -c user-container -f
processed event with ID: 1
processed event with ID: 9
processed event with ID: 6
processed event with ID: 2
processed event with ID: 10
processed event with ID: 7
processed event with ID: 8
processed event with ID: 3
processed event with ID: 4
processed event with ID: 5
```

#### Cleanup
Delete the trigger-demo namespace to easily remove all the created resources

```sh
kubectl delete -f samples/trigger-customizations/100-namespace.yaml
```
or
```sh
kubectl delete ns trigger-demo
```
