# RabbitMQ Knative Eventing Broker

![RabbitMQ Broker for Knative Eventing](rabbitmq-knative-broker.png)

## Prerequisites

install Knative Serving and Eventing as documented
[here](https://knative.dev/docs/install/any-kubernetes-cluster/)

Install the RabbitMQ Operator as documented
[here](https://github.com/rabbitmq/cluster-operator).

Create a RabbitMQ Cluster:

```
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rokn
  namespace: default
spec:
  replicas: 1
EOF
```

create a secret containing the brokerURL for that cluster (skip this if using
_way two_):

```sh
R_USERNAME=$(kubectl get secret --namespace default rokn-rabbitmq-admin -o jsonpath="{.data.username}" | base64 --decode)
R_PASSWORD=$(kubectl get secret --namespace default rokn-rabbitmq-admin -o jsonpath="{.data.password}" | base64 --decode)

kubectl create secret generic rokn-rabbitmq-broker-secret \
    --from-literal=brokerURL="amqp://$R_USERNAME:$R_PASSWORD@rokn-rabbitmq-client.default:5672"
```

You can also optionally install
[KEDA based autoscaler](https://github.com/knative-sandbox/eventing-autoscaler-keda).

## Installation

You must have [`ko`](https://github.com/google/ko) installed. Then install the
broker-controller from this repository:

```
ko apply -f config/broker/
```

## Demo

create a broker:

```
kubectl apply -f - << EOF
  apiVersion: eventing.knative.dev/v1
  kind: Broker
  metadata:
    name: default
    annotations:
      eventing.knative.dev/broker.class: RabbitMQBroker
  spec:
    config:
      apiVersion: v1
      kind: Secret
      name: rokn-rabbitmq-broker-secret
EOF
```

or _way two_:

```
kubectl apply -f - << EOF
  apiVersion: eventing.knative.dev/v1
  kind: Broker
  metadata:
    name: default
    annotations:
      eventing.knative.dev/broker.class: RabbitMQBroker
  spec:
    config:
      apiVersion: rabbitmq.com/v1beta1
      kind: RabbitmqCluster
      name: rokn
EOF
```

create a trigger:

```
kubectl apply -f - << EOF
  apiVersion: eventing.knative.dev/v1
  kind: Trigger
  metadata:
    name: ping-trigger
    namespace: default
  spec:
    broker: default
    filter:
      attributes:
        type: dev.knative.sources.ping
    subscriber:
      ref:
        apiVersion: serving.knative.dev/v1
        kind: Service
        name: subscriber
EOF
```

create a Ping Source:

```
kubectl apply -f - << EOF
  apiVersion: sources.knative.dev/v1alpha2
  kind: PingSource
  metadata:
    name: ping-source
  spec:
    schedule: "*/1 * * * *"
    jsonData: '{"message": "Hello world!"}'
    sink:
      ref:
        apiVersion: eventing.knative.dev/v1beta1
        kind: Broker
        name: default
EOF
```

create an event_display subscriber:

```
kubectl apply -f - << EOF
  apiVersion: serving.knative.dev/v1
  kind: Service
  metadata:
    name: subscriber
    namespace: default
  spec:
    template:
      spec:
        containers:
        - image: gcr.io/knative-releases/knative.dev/eventing-contrib/cmd/event_display
EOF
```

tail the logs on the subscriber's depoyment, and you should see a "Hello world!"
event once per minute; for example using [kail](https://github.com/boz/kail):

```sh
$ kail -d subscriber-4kf8l-deployment
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]: ☁️  cloudevents.Event
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]: Validation: valid
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]: Context Attributes,
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   specversion: 1.0
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   type: dev.knative.sources.ping
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   source: /apis/v1/namespaces/default/pingsources/ping-source
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   id: 1fec78d7-20c2-459f-ac5e-8a797ca7bcdd
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   time: 2020-05-13T17:19:00.000374701Z
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   datacontenttype: application/json
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]: Data,
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   {
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:     "message": "Hello world!"
default/subscriber-4kf8l-deployment-8d556d6cd-j669x[user-container]:   }
```
