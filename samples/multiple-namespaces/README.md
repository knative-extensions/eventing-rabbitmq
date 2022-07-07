# RabbitMQ Knative Eventing Multi-Namespaces Cluster Example

## Prerequisites

- Same as listed [here](../../../docs/source.md#prerequisites)

## Overview

What this demo shows is how to manage multiple namespaces with one single RabbitMQ Cluster instance by creating 2 Broker-Trigger topologies in different namespaces, but connected to the same RabbitMQ Cluster and sending events through them.

## Steps

### Create the namespaces

```sh
kubectl apply -f samples/multiple-namespaces/100-namespaces.yaml
```
or
```sh
kubectl ns create ping pong
```

### Create a RabbitMQ Cluster

Create a RabbitMQ Cluster:

```sh
kubectl apply -f samples/multiple-namespaces/200-rabbitmq-cluster.yaml
```

### Create the RabbitMQ Trigger Broker Topologies

```sh
kubectl apply -f samples/multiple-namespaces/300-ping-events.yaml
kubectl apply -f samples/multiple-namespaces/400-pong-events.yaml
```

### Check the results

Check the CloudEvents players in both namespaces

```sh
kubectl -n ping -l='serving.knative.dev/service=ping-cloudevents-player-rabbitmq' logs -c user-container
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: dev.knative.sources.ping
  source: /apis/v1/namespaces/ping/ping-source
  id: 166e89ff-19c7-4e9a-a593-9ed30dca0d7d
  time: 2020-10-06T10:35:00.307531386Z
  datacontenttype: application/json
Data,
  {
    ...
  }
```
```sh
kubectl -n pong -l='serving.knative.dev/service=pong-cloudevents-player-rabbitmq' logs -c user-container
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: dev.knative.sources.ping
  source: /apis/v1/namespaces/pong/pong-source
  id: 166e89ff-19c7-4e9a-a593-9ed30dca0d7d
  time: 2020-10-06T10:35:00.307531386Z
  datacontenttype: application/json
Data,
  {
    ...
  }
```

### Cleanup

```sh
kubectl delete -f samples/multiple-namespaces/
```

