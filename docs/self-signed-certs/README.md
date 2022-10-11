# Eventing RabbitMQ with Self-Signed Certificates

This document outlines the steps needed to use Eventing RabbitMQ on an instance of RabbitMQ with self-signed certificates. If your RabbitMQ instance has a valid signed certificate, then follow the instructions [here](../../samples/rabbitmq-mtls/).

## Certificate Requirements

The certificate will need to have additional Subject Alternative Names (SAN) if using Eventing RabbitMQ with the cluster operator:

- <RABBITMQ_CLUSTER_NAME>.<RABBITMQ_CLUSTER_NAMESPACE>.svc
- <RABBITMQ_CLUSTER_NAME>.<RABBITMQ_CLUSTER_NAMESPACE>.svc.<Kubernetes Cluster Domain>

Create a Kubernetes Secret with the data from the server certificate and private key. The <NAMESPACE> should be the namespace of the RabbitMQ Cluster if using cluster operator or the namespace of the Secret when connecting to an external RabbitMQ instance:
```
kubectl -n <NAMESPACE> create secret tls <TLS_SECRET_NAME> --cert=<path-to-server-cert> --key=<path-to-server-key>
```

Create a Kubernetes Secret in the same namespace as the RabbitMQ cluster with the CA certificate of the signing entity:
```
kubectl -n <NAMESPACE> create secret generic <CA_SECRET_NAME> --from-file=ca.crt=<path-to-ca-crt>
```

## Messaging Topology Operator Setup

Follow the instruction [here](https://www.rabbitmq.com/kubernetes/operator/tls-topology-operator.html) to patch the messaging topology operator deployment to trust the CA certificate.

## Cluster Operator Setup

The RabbitMQ cluster's spec will need to include references to the TLS Secret as well as the CA certificate Secret for a self-signed setup:
```
apiVersion: rabbitmq.com/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmq
  namespace: quick-setup-sample
spec:
  tls:
    caSecretName: <CA_SECRET_NAME>
    secretName: <TLS_SECRET_NAME>
    disableNonTLSListeners: true
  ...
```

More information on configuring TLS for Cluster Operator based RabbitMQ Clusters can be found [here](https://www.rabbitmq.com/kubernetes/operator/using-operator.html#tls).

## External RabbitMQ Setup

The Secret containing the connection details for the external RabbitMQ instance will also need to have a reference to the CA_SECRET:

```
apiVersion: v1
kind: Secret
metadata:
  name: rabbitmq-secret-credentials
  namespace: external-cluster-sample
# This is just a sample, don't use it this way in production
stringData:
  ...
  caSecretName: <CA_SECRET_NAME>
  ...
```
