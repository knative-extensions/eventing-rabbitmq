# Tests

General Knative Eventing testing topics covered in [Knative Eventing Tests](https://github.com/knative/eventing/blob/main/test/README.md)

## Testing Eventing RabbitMQ

Eventing RabbitMQ testing workflow is centered around Makefile.
Things like initializing and setting-up kind cluster, installing
components and running tests - everything done with `make`.

### Create environment

```
make .envrc && source .envrc
```

Here we create our environment file containing patched `PATH`, various exports, etc.
[Direnv](https://direnv.net/) will pick up `.envrc` and source it
each time shell visits Eventing RabbitMQ folder.

### Create kind cluster

```
make kind-cluster
```

This will install `kind` to ./bin and initialize the cluster. `Kind` uses [kind.yaml](../test/e2e/kind.yaml) as cluster configuration.


### Install Knative, Certificate manager, RabbitMQ

```
make install
```

### k9s

```
make k9s
```

[k9s](https://k9scli.io/) is a "Kubernetes CLI To Manage Your Clusters In Style". That `make` target takes care of `k9s` installation and configuration.
This target will run `k9s` pointed to our kind cluster.

### Running tests

At the time of writing the following targets available:

```
test-compilation                    Build test binaries with e2e tags
test-conformance                    Run conformance tests
test-e2e                            Run all end-to-end tests - manages all dependencies, including K8S components
test-e2e-broker                     Run Broker end-to-end tests - assumes a K8S with all necessary components installed (Knative & RabbitMQ)
test-e2e-publish                    Run TestKoPublish end-to-end tests  - assumes a K8S with all necessary components installed (Knative & RabbitMQ)
test-e2e-source                     Run Source end-to-end tests - assumes a K8S with all necessary components installed (Knative & RabbitMQ)
test-unit                           Run unit tests
test-unit-uncached                  Run unit tests with no cache
```

## Temporary files

Makefile will download and store binaries such as `kind`, `k9s`, `kubectl` to `(curdir)/bin`.
Configurations will be stored in `(curdir)/.config`.
Both paths are in the `.gitignore`.

## How to start fresh

```
make reset
```
