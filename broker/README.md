# RabbitMQ Knative Eventing Broker

RabbitMQ *is a Messaging Broker* - an intermediary for messaging. It gives your applications a common platform to send and receive messages, and your messages a safe place to live until received.

![RabbitMQ Broker for Knative Eventing](rabbitmq-knative-broker.png)

# Table of Contents

- [Installation](#installation)
  - [Standalone Broker](#standalone-broker)
  - [RabbitMQ Operator Based Broker](#rabbitmq-operator-based-broker)
- [Autoscaling](#autoscaling-optional)
- [Next Steps](#next-steps)
- [Additional Resources](#additional-resources)

## Installation

We provide two versions of the RabbitMQ Knative Eventing Broker.

1. [Standalone Broker](#standalone-broker)
2. [RabbitMQ operator based Broker](#rabbitmq-operator-based-broker)

### Standalone Broker

This Broker works by utilizing libraries to manage RabbitMQ resources directly and as name implies does not have have dependencies on other operators. Choose this if you do not manage your clusters lifecycle with the [Cluster Operator](https://github.com/rabbitmq/cluster-operator).

[Install Standalone Broker](./standalone.md)

### RabbitMQ Operator Based Broker

This Broker builds on top of [Cluster Operator](https://github.com/rabbitmq/cluster-operator) and [Messaging Topology Operator](https://github.com/rabbitmq/messaging-topology-operator). As such it requires both of them to be installed. This Broker also will only work with RabbitMQ clusters created and managed by the Cluster Operator and as such if you do not manage your RabbitMQ clusters with it, you must use the [Standalone Broker](./standalone.md).

[Install Operator based Broker](./operator-based.md)

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

More details and samples can be found [here](../samples/trigger-customizations)

## Next Steps

- Check out the [Broker-Trigger Samples Directory](../samples/broker-trigger) in this repo and start building your topology with Eventing RabbitMQ!

## Additional Resources

- [RabbitMQ Docs](https://www.rabbitmq.com/documentation.html)
- [RabbitMQ Operator Docs](https://www.rabbitmq.com/kubernetes/operator/operator-overview.html)
- [Knative Docs](https://knative.dev/docs/)
