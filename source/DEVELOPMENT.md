# RabbitMQ - Source

The Rabbitmq Event source enables Knative Eventing integration with Rabbitmq.
When an message is produced to Rabbitmq, the Rabbitmq Event Source will consume
the produced message and post that message the corresponding event sink.

This sample demonstrates how to configure, deploy, and use the Rabbitmq Event
Source with a Knative Service.

## Prerequisites

* Follow the [Operator based Broker's Prerequisites Section](../broker/operator-based.md#prerequisites)

* Before we can create the Knative Eventing Source, we first need to create a RabbitMQ Cluster:

```shell
kubectl apply -f - << EOF
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmq
  namespace: default
spec:
  replicas: 1
EOF
```

## Installation

You can install the latest released version of the [Knative RabbitMQ Source](https://github.com/knative-sandbox/eventing-rabbitmq/releases/):

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/latest/download/rabbitmq-source.yaml
```

If you wanted to install a specific version, e.g. v0.25.0, you can run:

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/download/v0.25.0/rabbitmq-source.yaml
```

You can install a nightly version:

```shell
kubectl apply -f https://storage.googleapis.com/knative-nightly/eventing-rabbitmq/latest/rabbitmq-source.yaml
```

Or if you want to run the latest version from this repo, you can use [`ko`](https://github.com/google/ko) to install it.

- Install the `ko` CLI for building and deploying purposes.

   ```shell script
   go get github.com/google/go-containerregistry/cmd/ko
   ```

- Configure container registry, such as a Docker Hub account, is required.

- Export the `KO_DOCKER_REPO` environment variable with a value denoting the
   container registry to use.

   ```shell script
   export KO_DOCKER_REPO="docker.io/YOUR_REPO"
   ```
- Install the source operator
   ```
   ko apply -f config/source/
   ```

Now you can create a RabbitMQ source in the default namespace running:
```sh
kubectl apply -f - << EOF
apiVersion: sources.knative.dev/v1alpha1
kind: RabbitmqSource
metadata:
  name: rabbitmq-source
spec:
  brokers: "rabbitmq:5672/"
  user:
    secretKeyRef:
      name: rabbitmq-default-user
      key: username
  password:
    secretKeyRef:
      name: rabbitmq-default-user
      key: password
  channel_config:
    global_qos: false
  exchange_config:
    name: "eventing-rabbitmq-source"
    type: "fanout"
  queue_config:
    name: "eventing-rabbitmq-source"
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: rabbitmq-source-sink
      namespace: source-demo
EOF
```

## Build and Deployment

The following steps build and deploy the Rabbitmq Event Controller, Source, and
an Event Display Service.

- Assuming current working directory is the project root `eventing-sources`.

#### Rabbitmq Event Controller

1. Check that the `rabbitmq-controller-manager` pod is running. The name of the
   pod may be different than shown here.

   ```shell script
   $ kubectl get pods -n knative-sources
   NAME                         READY     STATUS    RESTARTS   AGE
   rabbitmq-controller-manager-0   1/1       Running   0          42m
   ```

2. Check the `controller-manager-0` pod logs.

   ```shell script
   $ kubectl logs -l 'control-plane=rabbitmq-controller-manager' -n knative-sources
   2019/03/19 22:25:54 Registering Components.
   2019/03/19 22:25:54 Setting up Controller.
   2019/03/19 22:25:54 Adding the Rabbitmq Source controller.
   2019/03/19 22:25:54 Starting Rabbitmq controller.
   ```

#### Rabbitmq Event Source

1. Optionally a secret named `rabbitmq-source-key` can be created to store rabbitmq broker
   credentials with following command

   ```shell script
   kubectl create secret generic rabbitmq-source-key -n source-demo --from-literal=user=guest --from-literal=password=guest
   ```

2. Modify `contrib/rabbitmq/samples/event-source.yaml` accordingly with brokers,
   topic, routing key etc...

3. Configure the event source parameters.

   - Configure channel config properties based on this documentation.

     ```
     1. Qos controls how many messages or how many bytes the server will try to keep on
     the network for consumers before receiving delivery acks.  The intent of Qos is
     to make sure the network buffers stay full between the server and client.

     2. With a prefetch count greater than zero, the server will deliver that many
     messages to consumers before acknowledgments are received.  The server ignores
     this option when consumers are started with noAck because no acknowledgments
     are expected or sent.

     3. When global is true, these Qos settings apply to all existing and future
     consumers on all channels on the same connection.  When false, the Channel.Qos
     settings will apply to all existing and future consumers on this channel.

     4. Please see the RabbitMQ Consumer Prefetch documentation for an explanation of
     how the global flag is implemented in RabbitMQ, as it differs from the
     AMQP 0.9.1 specification in that global Qos settings are limited in scope to
     channels, not connections (https://www.rabbitmq.com/consumer-prefetch.html).

     5. To get round-robin behavior between consumers consuming from the same queue on
     different connections, set the prefetch count to 1, and the next available
     message on the server will be delivered to the next available consumer.

     6. If your consumer work time is reasonably consistent and not much greater
     than two times your network round trip time, you will see significant
     throughput improvements starting with a prefetch count of 2 or slightly
     greater as described by benchmarks on RabbitMQ.

     7. http://www.rabbitmq.com/blog/2012/04/25/rabbitmq-performance-measurements-part-2/
     ```

   - Configure exchange config properties based on this documentation.

     ```
     1. Exchange names starting with "amq." are reserved for pre-declared and
     standardized exchanges. The client MAY declare an exchange starting with
     "amq." if the passive option is set, or the exchange already exists.  Names can
     consist of a non-empty sequence of letters, digits, hyphen, underscore,
     period, or colon.

     2. Each exchange belongs to one of a set of exchange kinds/types implemented by
     the server. The exchange types define the functionality of the exchange - i.e.
     how messages are routed through it. Once an exchange is declared, its type
     cannot be changed.  The common types are "direct", "fanout", "topic" and
     "headers".

     3. Durable and Non-Auto-Deleted exchanges will survive server restarts and remain
     declared when there are no remaining bindings.  This is the best lifetime for
     long-lived exchange configurations like stable routes and default exchanges.

     4. Non-Durable and Auto-Deleted exchanges will be deleted when there are no
     remaining bindings and not restored on server restart.  This lifetime is
     useful for temporary topologies that should not pollute the virtual host on
     failure or after the consumers have completed.

     5. Non-Durable and Non-Auto-deleted exchanges will remain as long as the server is
     running including when there are no remaining bindings.  This is useful for
     temporary topologies that may have long delays between bindings.

     6. Durable and Auto-Deleted exchanges will survive server restarts and will be
     removed before and after server restarts when there are no remaining bindings.
     These exchanges are useful for robust temporary topologies or when you require
     binding durable queues to auto-deleted exchanges.

     7. Note: RabbitMQ declares the default exchange types like 'amq.fanout' as
     durable, so queues that bind to these pre-declared exchanges must also be
     durable.

     8. Exchanges declared as `internal` do not accept accept publishings. Internal
     exchanges are useful when you wish to implement inter-exchange topologies
     that should not be exposed to users of the broker.

     9. When noWait is true, declare without waiting for a confirmation from the server.
     The channel may be closed as a result of an error.  Add a NotifyClose listener
     to respond to any exceptions.

     10. Optional amqp.Table of arguments that are specific to the server's implementation of
     the exchange can be sent for exchange types that require extra parameters.
     ```

   - Configure queue config properties based on this documentation.

     ```
     1. The queue name may be empty, in which case the server will generate a unique name
     which will be returned in the Name field of Queue struct.

     2. Durable and Non-Auto-Deleted queues will survive server restarts and remain
     when there are no remaining consumers or bindings.  Persistent publishings will
     be restored in this queue on server restart.  These queues are only able to be
     bound to durable exchanges.

     3. Non-Durable and Auto-Deleted queues will not be redeclared on server restart
     and will be deleted by the server after a short time when the last consumer is
     canceled or the last consumer's channel is closed.  Queues with this lifetime
     can also be deleted normally with QueueDelete.  These durable queues can only
     be bound to non-durable exchanges.

     4. Non-Durable and Non-Auto-Deleted queues will remain declared as long as the
     server is running regardless of how many consumers.  This lifetime is useful
     for temporary topologies that may have long delays between consumer activity.
     These queues can only be bound to non-durable exchanges.

     5. Durable and Auto-Deleted queues will be restored on server restart, but without
     active consumers will not survive and be removed.  This Lifetime is unlikely
     to be useful.

     6. Exclusive queues are only accessible by the connection that declares them and
     will be deleted when the connection closes.  Channels on other connections
     will receive an error when attempting  to declare, bind, consume, purge or
     delete a queue with the same name.

     7. When noWait is true, the queue will assume to be declared on the server.  A
     channel exception will arrive if the conditions are met for existing queues
     or attempting to modify an existing queue from a different connection.
     ```

4. Build and deploy the event source.

   ```shell script
   $ ko apply -f samples/source/500-source.yaml
   ...
   rabbitmqsource.sources.eventing.knative.dev/rabbitmq-source created
   ```

5. Check that the event source pod is running. The pod name will be prefixed
   with `rabbitmq-source`.

   ```shell script
   $ kubectl get pods -n source-demo
   NAME                                  READY     STATUS    RESTARTS   AGE
   rabbitmq-source-xlnhq-5544766765-dnl5s   1/1       Running   0          40m
   ```

6. Ensure the Rabbitmq Event Source started with the necessary configuration.

   ```shell script
   $ kubectl logs -l "eventing.knative.dev/source=rabbitmq-source-controller"
   {"level":"info","ts":"2019-04-09T23:09:59.156Z","caller":"receive_adapter/main.go:112","msg":"Starting Rabbitmq Receive Adapter...","adapter":{"Brokers":"amqp://guest:guest@rabbitmq:5672/","Topic":"","ExchangeConfig":{"Name":"","TypeOf":"fanout","Durable":true,"AutoDeleted":false,"Internal":false,"NoWait":false},"QueueConfig":{"Name":"","RoutingKey":"","Durable":false,"DeleteWhenUnused":false,"Exclusive":false,"NoWait":false},"SinkURI":"http://event-display.default.svc.cluster.local/"}}
   ```

#### Event Display

1. Build and deploy the Event Display Service.

   ```shell script
   $ ko apply -f samples/source/event-display.yaml
   ...
   service.serving.knative.dev/event-display created
   ```

2. Ensure that the Service pod is running. The pod name will be prefixed with
   `event-display`.

   ```shell script
   $ kubectl get pods
   NAME                                            READY     STATUS    RESTARTS   AGE
   event-display-00001-deployment-5d5df6c7-gv2j4   2/2       Running   0          72s
   ...
   ```

### Verify

1. Produce the message shown below to Rabbitmq. A simple producer is available
   [here](../samples/source/example/simple_producer.go)

   ```shell script
   "Hello World"
   ```

2. Check that the Rabbitmq Event Source consumed the message and sent it to its
   sink properly.

   ```shell script
   $ kubectl logs -l "eventing.knative.dev/source=rabbitmq-source-controller"
   ...
   {"level":"info","ts":1554244010.3225584,"logger":"fallback","caller":"adapter/adapter.go:158","msg":"Received: {value 15 0 Hello World <nil>}"}
   {"level":"info","ts":1554244016.0724912,"logger":"fallback","caller":"adapter/adapter.go:196","msg":"Successfully sent event to sink"}
   ```

3. Ensure the Event Display received the message sent to it by the Event Source.

   ```shell script
   $ kubectl logs -l "serving.knative.dev/service=event-display" -c user-container

   ☁️  cloudevents.Event
   Validation: valid
   Context Attributes,
     specversion: 1.0
     type: dev.knative.rabbitmq.event
     source: /apis/v1/namespaces/default/rabbitmqsources/rabbitmq-source
     id: f00c1f52-33a1-4d3d-993f-750f20c804da
     time: 2020-12-18T01:15:20.450860898Z
     datacontenttype: application/json
   Extensions,
     key:
   Data,
     Hello rabbitmq!
   ☁️  cloudevents.Event
   Validation: valid
   Context Attributes,
     specversion: 1.0
     type: dev.knative.rabbitmq.event
     source: /apis/v1/namespaces/default/rabbitmqsources/rabbitmq-source
     id: 81472c5a-7901-4d03-bbfb-00b7ca852d5f
     time: 2020-12-18T01:15:21.668571977Z
     datacontenttype: application/json
   Extensions,
     key:
   Data,
     Hello rabbitmq!
   ```

## Teardown Steps

1. Remove the Rabbitmq Event Source

   ```shell script
   $ ko delete -f contrib/rabbitmq/samples/source.yaml
   rabbitmqsource.sources.eventing.knative.dev "rabbitmq-source" deleted
   ```

2. Remove the Event Display

   ```shell script
   $ ko delete -f contrib/rabbitmq/samples/event-display.yaml
   service.serving.knative.dev "event-display" deleted
   ```

3. Remove the Rabbitmq Event Controller

   ```shell script
   $ ko delete -f contrib/rabbitmq/config
   serviceaccount "rabbitmq-controller-manager" deleted
   clusterrole.rbac.authorization.k8s.io "eventing-sources-rabbitmq-controller" deleted
   clusterrolebinding.rbac.authorization.k8s.io "eventing-sources-rabbitmq-controller" deleted
   customresourcedefinition.apiextensions.k8s.io "rabbitmqsources.sources.eventing.knative.dev" deleted
   service "rabbitmq-controller" deleted
   statefulset.apps "rabbitmq-controller-manager" deleted
   ```
