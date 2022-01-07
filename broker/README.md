# RabbitMQ Knative Eventing Broker

RabbitMQ *is a Messaging Broker* - an intermediary for messaging. It gives your applications a common platform to send and receive messages, and your messages a safe place to live until received.

![RabbitMQ Broker for Knative Eventing](rabbitmq-knative-broker.png)

# Table of Contents

- [Installation](#installation)
  - [Standalone Broker](#standalone-broker)
  - [RabbitMQ Operator Based Broker](#rabbitmq-operator-based-broker)
- [Autoscaling](#autoscaling)
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

## Next Steps

Check out the [Broker-Trigger Samples Directory](../samples/broker-trigger) in this repo and start building your topology with Eventing RabbitMQ!

## Additional Resources

- [RabbitMQ Docs](https://www.rabbitmq.com/documentation.html)
- [Knative Docs](https://knative.dev/docs/)
