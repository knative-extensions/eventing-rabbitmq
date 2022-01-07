# RabbitMQ Source

The RabbitMQ source translates messages on a RabbitMQ exchange to CloudEvents
based on the https://github.com/knative-sandbox/eventing-rabbitmq/blob/main/cloudevents-protocol-spec/spec.md[RabbitMQ Protocol Binding for CloudEvents Spec],
which can then be used with Knative Eventing over HTTP. The source can bind to
an existing RabbitMQ exchange, or create a new exchange if required.

# Table of Contents

- [Published Events](#published-events)
- [Samples](#samples)
- [Creating and Managing Sources](#creating-and-managing-sources)
- [Administration](#administration)
  - [Prerequisites](#prerequisites)
  - [Administration](#administration)
  - [Configuration options](#configuration-options)
- [Next Steps](#next-steps)
- [Additional Resources](#additional-resources)

## Published Events

All messages received by the source are published with the following schema:

Event attributes
| Attribute | Value  | Notes  |
|-----------|--------|--------|
| `type` | `dev.knative.rabbitmq.event` | |
| `source` | `/apis/v1/namespace/*$NS*/rabbitmqsources/*$NAME*#*$TOPIC*` | `NS`, `NAME` and `TOPIC` are derived from the source configuration |
| `id` | A unique ID | This uses the `MessageId` if available, and a UUID otherwise |
| `subject` | The ID of the message | Empty string if no message ID is present |
| `datacontenttype` | `application/json` | Currently static |
| `key` | The ID of the message | Empty string if no message ID is present |

The payload of the event is set to the data content of the message.

## Samples

For a message published with the payload "Hello rabbitmq!", for example with
[`rabbitmqadmin`](https://www.rabbitmq.com/management-cli.html):

```sh
rabbitmqadmin publish exchange=amq.default payload="Hello rabbitmq!"
```

The source sends the following event content:

.CloudEvents JSON format

```json
{
  "specversion": "1.0",
  "type": "dev.knative.rabbitmq.event",
  "source": "/apis/v1/namespaces/default/rabbitmqsources/rabbitmq-source",
  "id": "f00c1f52-33a1-4d3d-993f-750f20c804da",
  "time": "2020-12-18T01:15:20.450860898Z",
  "subject": "",
  "datacontenttype": "application/json",
  "key": "",
  "data": "Hello rabbitmq!"
}
```

## Creating and Managing Sources

Sources are Kubernetes objects. In addition to the standard Kubernetes
`apiVersion`, `kind`, and `metadata`, they have the following `spec` fields:

Source parameters
| Field  | Value  |
|--------|--------|
| `spec.brokers` | Host+Port of the Broker, with a trailing "/" |
| `spec.vhost` * | VHost where the source resources are located |
| `spec.predeclared` | Defines if the source should try to create new queue or use predeclared one (Boolean) |
| `user.secretKeyRef` | Username for Broker authentication; field `key` in a Kubernetes Secret named `name` |
| `password.secretKeyRef` | Password for Broker authentication; field `key` in a Kubernetes Secret named `name` |
| `topic` | The topic for the exchange |
| `exchange_config` | Settings for the exchange |
| `exchange_config.type` | [Exchange type](https://www.rabbitmq.com/tutorials/amqp-concepts.html#exchanges). Can be `fanout`, `direct`, `topic`, `match` or `headers` |
| `exchange_config.durable` * | Boolean |
| `exchange_config.auto_deleted` * | Boolean |
| `exchange_config.internal` * | Boolean |
| `exchange_config.nowait` * | Boolean |
| `queue_config` * | Settings for the queue |
| `queue_config.name` * | Name of the queue (may be empty) |
| `queue_config.routing_key` * | Routing key for the queue |
| `queue_config.durable` * | Boolean |
| `queue_config.delete_when_unused` * | Boolean |
| `queue_config.exclusive` * | Boolean |
| `queue_config.nowait` * | Boolean |
| `channel_config.prefetch_count` * | Int that limits the [Consumer Prefetch Value](https://www.rabbitmq.com/consumer-prefetch.html). Default value is `1`. Value must be between `1` and `1000`. With a value of `1` the RabbitMQ Source process events in FIFO order, values above `1` break message ordering guarantees and can be seen as more performance oriented. |
| `channel_config.global_qos` * | Boolean defining how the [Consumer Sharing Limit](https://www.rabbitmq.com/consumer-prefetch.html#sharing-the-limit) is handled. |
| `sink` | A reference to an [Addressable](https://knative.dev/docs/eventing/#event-consumers) Kubernetes object |

`*` These attributes are optional.

You will need a Kubernetes Secret to hold the RabbitMQ username and
password. The following command is one way to create a secret with the username
`rabbit-user` and the password taken from the `/tmp/password` file.

```sh
kubectl create secret generic rabbitmq-secret \
  --from-literal=user=rabbit-user \
  --from-file=password=/tmp/password
```

Note that many parameters do not need to be specified. Unspecified optional
parameters will be defaulted to `false` or `""` (empty string).

```yaml
apiVersion: sources.knative.dev/v1alpha1
kind: RabbitmqSource
metadata:
  name: rabbitmq-source
spec:
  brokers: "rabbitmq:5672/"
  user:
    secretKeyRef:
      name: "rabbitmq-secret"
      key: "user"
  password:
    secretKeyRef:
      name: "rabbitmq-secret"
      key: "password"
  exchange_config:
    type: "fanout"
    durable: true
    auto_deleted: false
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: event-display
```

The Source will provide output information about readiness or errors via the
`status` field on the object once it has been created in the cluster.

<!---// TODO: should we have error documentation?--->

## Administration

The following information is intended for kubernetes cluster administrators
looking to install the RabbitMQ source.

### Prerequisites

* A [RabbitMQ](https://www.rabbitmq.com/) installation. On Kubernetes, you can use
[the RabbitMQ
operator](https://www.rabbitmq.com/kubernetes/operator/operator-overview.html) to set up a RabbitMQ installation.

* An understanding of RabbitMQ concepts like Brokers, Exchanges, and Queues.

### Installation

*  Install the source from the latest release:
```sh
kubectl apply -f https://github.com/knative-sandbox/eventing-rabbitmq/releases/latest/download/rabbitmq-source.yaml
```

Go to the [Development Guide](./DEVELOPMENT.md) for more detailed info about installation and basic configuration.

### Configuration Options

The standard
[`config-observability`](https://github.com/knative/eventing/blob/master/config/core/configmaps/observability.yaml)
and
[`config-logging`](https://github.com/knative/eventing/blob/master/config/core/configmaps/logging.yaml)
ConfigMaps may be used to manage the logging and metrics configuration.

## Next Steps

Check out the [Source Samples Directory](../samples/source) in this repo and start converting your messages to CloudEvents with Eventing RabbitMQ!

## Additional Resources

- [RabbitMQ Docs](https://www.rabbitmq.com/documentation.html)
- [Knative Docs](https://knative.dev/docs/)
