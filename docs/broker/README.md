# Knative Eventing RabbitMQ Broker

RabbitMQ *is a Messaging Broker* - an intermediary for messaging. It gives your applications a common platform to send and receive messages, and your messages a safe place to live until received.

![RabbitMQ Broker for Knative Eventing](rabbitmq-knative-broker.png)

# Table of Contents

- [Installation](#installation)
- [Autoscaling](#autoscaling-optional)
- [Next Steps](#next-steps)
- [Additional Resources](#additional-resources)

## Installation
### Prerequisites

* Install Knative Eventing as documented [here](https://knative.dev/docs/install/).

* Install latest released version of the [RabbitMQ Cluster Operator](https://github.com/rabbitmq/cluster-operator).

    ```
    kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml
    ```

* Install latest released version of the [RabbitMQ Messaging Topology Operator](https://github.com/rabbitmq/messaging-topology-operator)

    Cert Manager is a pre-requisite for the Topology Operator:

    ```
    kubectl apply -f https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
    ```

    Install the messaging topology operator:

    ```
    kubectl apply -f https://github.com/rabbitmq/messaging-topology-operator/releases/latest/download/messaging-topology-operator-with-certmanager.yaml
    ```

    If a custom installation is required, refer to the [topology operator docs](https://github.com/rabbitmq/messaging-topology-operator#quickstart).

### Install rabbitmq-eventing broker

Install the latest version of the [Operator based Knative RabbitMQ Broker](https://github.com/knative-sandbox/eventing-rabbitmq/releases/):

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/latest/download/rabbitmq-broker.yaml
```

Or install a specific version, e.g., v0.25.0, run:

```shell
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/download/v0.25.0/rabbitmq-broker.yaml
```

Or install a nightly version:

```shell
kubectl apply -f https://storage.googleapis.com/knative-nightly/eventing-rabbitmq/latest/rabbitmq-broker.yaml
```

For development purposes or to use the latest from the repository, use [`ko`](https://github.com/google/ko) for installation from a local copy of the repository.

```
ko apply -f config/broker/
```

Follow the [Broker-Trigger](../samples/broker-trigger) to deploy a basic example of a topology.

## Autoscaling (optional)

To get autoscaling (scale to zero as well as up from 0), you can also optionally
install
[KEDA based autoscaler](https://github.com/knative-sandbox/eventing-autoscaler-keda).

## Trigger Pre-Fetch Count
Trigger has a configurable annotation `rabbitmq.eventing.knative.dev/parallelism`. The following are effects of setting this parameter to `n`:

- Prefetch count is set to this value on the RabbitMQ channel and queue created for this trigger. The channel will receive a maximum of `n` number of messages at once.
- The trigger will create `n` workers to consume messages off the queue and dispatch to the sink.

If this value is unset, it will default to `1`. This means the trigger will only handle one event at a time. This will preserve the order of the messages in a queue but
will make the trigger a bottleneck. A slow processing sink will result in low overall throughput. Setting a value higher than 1 will result in `n` events being handled at
a time by the trigger but ordering won't be guaranteed as events are sent to the sink.

More details and samples can be found [here](../../samples/trigger-customizations)

## Next Steps
- Check out the [Broker-Trigger Samples Directory](../samples/broker-trigger) in this repo and start building your topology with Eventing RabbitMQ!
- Follow [CloudEvents Player Source](https://knative.dev/docs/getting-started/first-source/) to setup a Knative Service as a source.

## Additional Resources

- [RabbitMQ Docs](https://www.rabbitmq.com/documentation.html)
- [RabbitMQ Operator Docs](https://www.rabbitmq.com/kubernetes/operator/operator-overview.html)
- [Knative Docs](https://knative.dev/docs/)
