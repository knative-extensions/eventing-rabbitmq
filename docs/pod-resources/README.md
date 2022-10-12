# Configuring Pod Resources

When topologies with Brokers, Trigger and RabbitMQSources are created, they create exclusive pods to handle ingress and egress. For example, a Broker will create an ingress pod that will be used as an entry point to publish an event. Triggers will create dispatcher (egress) pods that are responsible for sending events to the Sink. This document outlines how a user can configure the CPU and memory resources for those pods.

## Available Annotations

The following annotations are availble to be attached to Brokers, Triggers and RabbitMQSources. Click [here](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) for more information on Kubernetes resource requirements.

| Annotation                                   | Usage                             |
|----------------------------------------------|-----------------------------------|
| rabbitmq.eventing.knative.dev/cpu-request    | Requested CPU resource units      |
| rabbitmq.eventing.knative.dev/cpu-limit      | Upper limit of CPU resource units |
| rabbitmq.eventing.knative.dev/memory-request | Requested memory                  |
| rabbitmq.eventing.knative.dev/memory-limit   | Upper limit for memory            |

## Configurable Kinds

The above annotations can be used on Brokers, Triggers and RabbitMQSources. The following table describes which pods will get affected when the annotations are used.

| Kind           | Pods Affected                                                                 |
|----------------|-------------------------------------------------------------------------------|
| Broker         | Ingress pod. DLQ dispatcher pod if a Dead Letter Queue (DLQ) is configured    |
| Trigger        | Dispatcher pod. DLQ dispatcher pod if a Dead Letter Queue (DLQ) is configured |
| RabbitMQSource | Receiver adapter pod.                                                         |

## Defaults

If resource requirements are not set, a default set of requirements will be present for each pod created. The defaults are based on performance benchmarks run with Eventing RabbitMQ. Note that setting any of the annotations will remove all defaulting for that pod. For example if `rabbitmq.eventing.knative.dev/memory-request` is set on a Broker, it will be the only resource requirement that will be present on the pod(s) created for the Broker.
