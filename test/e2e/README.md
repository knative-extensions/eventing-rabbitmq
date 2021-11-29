# E2E Tests

The following assumes some knowledge on running Knative tests. If this is your first time, see the [Eventing Tests](https://github.com/knative/eventing/blob/main/test/README.md) first before continuing. There you'll learn how to run the e2e suite, run specific tests, and create the required test images.

## Debugging a Failed Test

Each e2e test will run in its own namespace, and the tests clean themselves up when they finish. To keep test namespaces around, simply comment out the `env.Finish()` [line](https://github.com/knative-sandbox/eventing-rabbitmq/blob/main/test/e2e/main_test.go#L164) of the test you wish to focus on. Make sure you delete the namespaces when you're done.

## Profiling RabbitMQ

RabbitMQ has a dashboard with lots of information available about the cluster that can be useful for debugging tests.

1. Get the credentials out of a secret on the Kubernetes cluster where the tests are running:
  `kubectl -n <test namespace> get secrets rabbitmqc-default-user -o json | jq -r '.data["default_user.conf"]' | base64 -d`
2. Provided you are running your tests locally, you can port forward to get access to the RabbitMQ cluster:
  `kubectl -n TEST-NAMESPACE port-forward rabbitmqc-server-0 8081:15672`
3. Point your browser to http://localhost:8081 and log in with the credentials from step 1.
  **Note:** If the credentials don't work, you can also try `guest` for both username and password.

