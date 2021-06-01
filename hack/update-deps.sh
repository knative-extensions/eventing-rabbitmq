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

source $(dirname "$0")/../vendor/knative.dev/hack/library.sh

go_update_deps "$@"

rm -rf $(find third_party/ -name '*.mod')
rm -rf $(find third_party/ -name '*.go')

# Remove the _webhook.go files that cause (unnecessarily) controller-runtime
# to get pulled in, which in turn causes issues with double-defining 'kubeconfig'
# flag, which of course leads to all kinds of hilarity.
rm vendor/github.com/rabbitmq/messaging-topology-operator/api/v1beta1/*webhook.go
