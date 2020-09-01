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

echo "Starting End-to-End tests for Knative Eventing RabbitMQ API"

go test -v -tags=e2e -count=1 ${REPO_ROOT_DIR}/test/e2e
