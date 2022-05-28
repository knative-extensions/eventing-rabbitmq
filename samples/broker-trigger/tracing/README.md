# Configuring tracing with eventing-rabbitmq

For an overview of tracing with Knative in general, [this blog
post](https://knative.dev/blog/articles/distributed-tracing/) introduces the
basic concepts and methods for configuring Knative to emit traces.

eventing-rabbitmq facilitates tracing events through brokers and triggers by
attaching the `traceparent` and `tracestate` headers to the RabbitMQ messages
that are created by the `ingress` component of the broker and then converting
them back to HTTP headers before sending them to downstream sinks.

This means that all that is required to start collecting traces from
eventing-rabbitmq components is to configure your event source which sends events
to the broker to attach trace headers to the request.

This directory contains a sample broker/trigger topology similar to the one
found in the distributed tracing blog post, but with the additional creation of
a RabbitMQ Cluster and with the broker configured to use it. This sample
assumes you have set up the `otel-collector.observability` service as shown in
the blog post.
