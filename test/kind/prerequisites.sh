#!/usr/bin/env bash

# Copyright 2020 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit

readonly ROOT_DIR=$(dirname $0)/../..
[[ ! -v REPO_ROOT_DIR ]] && REPO_ROOT_DIR="$(git rev-parse --show-toplevel)"
readonly REPO_ROOT_DIR

readonly reg_port='5000'

export KO_DOCKER_REPO=localhost:${reg_port}

pwd

echo "Installing RabbitMQ Cluster Operator"

# TODO: The RabbitMQ team is working on releasing the yaml, https://github.com/rabbitmq/cluster-operator/issues/228

tmp_dir=$(mktemp -d -t ci-kind-XXX)
echo ${tmp_dir}

cd ${tmp_dir}

git clone https://github.com/rabbitmq/cluster-operator.git
cd cluster-operator

kubectl apply -f config/namespace/base/namespace.yaml
kubectl apply -f config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml
sleep 2 # Wait for the CRDs to be reconciled.
kubectl -n rabbitmq-system apply --kustomize config/rbac/
kubectl -n rabbitmq-system apply --kustomize config/manager/

echo "Installing KEDA"

tmp_dir=$(mktemp -d -t ci-kind-XXX)
echo ${tmp_dir}

cd ${tmp_dir}

git clone https://github.com/kedacore/keda && cd keda
git checkout v1.5.0

kubectl apply -f deploy/crds/keda.k8s.io_scaledobjects_crd.yaml
kubectl apply -f deploy/crds/keda.k8s.io_triggerauthentications_crd.yaml
sleep 2 # Wait for the CRDs to be reconciled.
kubectl apply -f deploy/

echo "Installing Knative Eventing"

kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.17.0/eventing-crds.yaml
sleep 2 # Wait for the CRDs to be reconciled.
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.17.0/eventing-core.yaml
