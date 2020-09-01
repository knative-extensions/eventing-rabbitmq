# RabbitMQ Knative Eventing Broker

![RabbitMQ Broker for Knative Eventing](rabbitmq-knative-broker.png)

## Prerequisites

install Knative Serving and Eventing as documented [here](https://knative.dev/docs/install/any-kubernetes-cluster/)

create a RabbitMQ cluster:
```sh
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install rokn bitnami/rabbitmq --set service.type=LoadBalancer
```

create a secret containing the brokerURL for that cluster:
```sh
PASSWORD=$(kubectl get secret --namespace default rokn-rabbitmq -o jsonpath="{.data.rabbitmq-password}" | base64 --decode)

kubectl create secret generic rokn-rabbitmq-broker-secret \
    --from-literal=brokerURL="amqp://user:$PASSWORD@rokn-rabbitmq.default:5672"
```

install KEDA as documented [here](https://keda.sh/docs/latest/deploy/).

## Installation

install the broker-controller from this repository:
```
ko apply -f config/
```

## Demo

create a broker:

```
kubectl apply -f - << EOF
  apiVersion: eventing.knative.dev/v1beta1
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

create a trigger:
```
kubectl apply -f - << EOF
  apiVersion: eventing.knative.dev/v1beta1
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

tail the logs on the subscriber's depoyment, and you should see a "Hello world!" event once per minute; for example using [kail](https://github.com/boz/kail):

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
