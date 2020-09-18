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

# Supported flags:
# Set an alternate name on the cluster, defaults to kink
#   --name "alternate-cluster-name"
# Verify the installation:
#   --check
#

# create registry container unless it already exists
cluster_name=knik
node_image='kindest/node:v1.18.8@sha256:f4bcc97a0ad6e7abaf3f643d890add7efe6ee4ab90baeb374b4f41a4c95567eb' # from the 0.9.0 release of kind.

kindVersion=`kind version`;

if [[ $kindVersion =~ "v0.9.0" ]]
then
   echo "KinD is v0.9.0"
else
  echo "Please make sure you are using KinD v0.9.0 or update the node_image"
  exit 0
fi

# Parse flags to determine any we should pass to dep.
check=0
shutdown=0
while [[ $# -ne 0 ]]; do
  parameter=$1
  case ${parameter} in
    --check) check=1 ;;
    --shutdown) shutdown=1 ;;
    *)
      [[ $# -ge 2 ]] || abort "missing parameter after $1"
      shift
      case ${parameter} in
        --name) cluster_name=$1 ;;
        *) abort "unknown option ${parameter}" ;;
      esac
  esac
  shift
done
readonly check
readonly shutdown
readonly cluster_name

kind_running="$(kind get clusters -q | grep ${cluster_name} || true)"

if (( check )); then
  if [[ "${kind_running}" ]]; then
    echo "KinD cluster ${cluster_name} is running"
  else
    echo "KinD cluster ${cluster_name} is NOT running"
  fi
  kind --version
  docker --version
  exit 0
fi

if (( shutdown )); then
  if [[ "${kind_running}" == "${cluster_name}" ]]; then
    echo "Deleting KinD cluster ${cluster_name}"
    kind delete cluster -q --name ${cluster_name}
  fi
  echo "Shutdown."
  exit 0
fi

# create a cluster
cat <<EOF | kind create cluster --name ${cluster_name} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: ${node_image}
- role: worker
  image: ${node_image}
EOF

echo "To use ko with kind:"
echo "export KO_DOCKER_REPO=kind.local"