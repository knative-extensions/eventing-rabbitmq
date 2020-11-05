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

export KO_DOCKER_REPO=kind.local
export KIND_CLUSTER_NAME=knik

pwd

echo "Installing RabbitMQ Cluster Operator"

kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/download/0.46.0/cluster-operator.yml

echo "Installing Knative Eventing"

kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.18.4/eventing-crds.yaml
sleep 2 # Wait for the CRDs to be reconciled.
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.18.4/eventing-core.yaml
