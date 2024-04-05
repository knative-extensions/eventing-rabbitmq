### Eventing RabbitMQ Setup Prerequisites

* Install Knative Eventing as documented [here](https://knative.dev/docs/install/)

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
