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
set -o nounset
set -o pipefail

source $(dirname $0)/../vendor/knative.dev/hack/codegen-library.sh

# If we run with -mod=vendor here, then generate-groups.sh looks for vendor files in the wrong place.
export GOFLAGS=-mod=

echo "=== Update Codegen for $MODULE_NAME"

group "Kubernetes Codegen"

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  knative.dev/eventing-rabbitmq/pkg/client knative.dev/eventing-rabbitmq/pkg/apis \
  "sources:v1alpha1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

# Only deepcopy the Duck types, as they are not real resources.
${CODEGEN_PKG}/generate-groups.sh "deepcopy" \
  knative.dev/eventing-rabbitmq/pkg/client knative.dev/eventing-rabbitmq/pkg/apis \
  "duck:v1beta1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

group "Knative Codegen"

# Knative Injection
${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
  knative.dev/eventing-rabbitmq/pkg/client knative.dev/eventing-rabbitmq/pkg/apis \
  "sources:v1alpha1 duck:v1beta1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

group "RabbitMQ Codegen"

# RabbitMQ uses Kubebuilder
# Kubebuilder project layout has API under 'api/v1beta1', ie. 'github.com/rabbitmq/messaging-topology-operator/api/v1beta1'
# client-go codegen expects group name (rabbitmq.com) in the path, ie. 'github.com/rabbitmq/messaging-topology-operator/api/rabbitmq.com/v1beta1'
# Because there's no way how to modify any of these settings, to enable client codegen,
# we need to hack things a little bit (in vendor move temporarily api directory in 'api/rabbitmq.com/v1beta1')
rm -rf ${REPO_ROOT_DIR}/vendor/github.com/rabbitmq/messaging-topology-operator/api/rabbitmq.com
mkdir ${REPO_ROOT_DIR}/vendor/github.com/rabbitmq/messaging-topology-operator/api/rabbitmq.com
cp -R "${REPO_ROOT_DIR}/vendor/github.com/rabbitmq/messaging-topology-operator/api/v1beta1/" ${REPO_ROOT_DIR}/vendor/github.com/rabbitmq/messaging-topology-operator/api/rabbitmq.com/v1beta1

OUTPUT_PKG="knative.dev/eventing-rabbitmq/pkg/client/injection/rabbitmq.com" \
VERSIONED_CLIENTSET_PKG="github.com/rabbitmq/messaging-topology-operator/pkg/generated/clientset/versioned" \
EXTERNAL_INFORMER_PKG="github.com/rabbitmq/messaging-topology-operator/pkg/generated/informers/externalversions" \
  ${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
    github.com/rabbitmq/messaging-topology-operator/rabbitmq.com \
    github.com/rabbitmq/messaging-topology-operator/api \
    "rabbitmq.com:v1beta1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt \

mv ${REPO_ROOT_DIR}/vendor/github.com/rabbitmq/messaging-topology-operator/api/rabbitmq.com/v1beta1 ${REPO_ROOT_DIR}/vendor/github.com/rabbitmq/messaging-topology-operator/api/v1beta1
rm -rf ${REPO_ROOT_DIR}/vendor/github.com/rabbitmq/messaging-topology-operator/api/rabbitmq.com

group "Update deps post-codegen"

# Make sure our dependencies are up-to-date
${REPO_ROOT_DIR}/hack/update-deps.sh
