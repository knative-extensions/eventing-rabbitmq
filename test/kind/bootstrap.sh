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
# Set registry port, defaults to 5000
#   --reg-port 1337
#

# create registry container unless it already exists
cluster_name=knik
reg_name='kind-registry'
reg_port='5000'
node_image='kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765'

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
        --reg-port) reg_port=$1 ;;
        *) abort "unknown option ${parameter}" ;;
      esac
  esac
  shift
done
readonly check
readonly shutdown
readonly cluster_name
readonly reg_name
readonly reg_port

reg_running="$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)"
kind_running="$(kind get clusters -q | grep ${cluster_name} || true)"

if (( check )); then
  if [[ "${kind_running}" ]]; then
    echo "KinD cluster ${cluster_name} is running"
  else
    echo "KinD cluster ${cluster_name} is NOT running"
  fi
  if [ "${reg_running}" == 'true' ]; then
    echo "Docker hosted registry ${reg_name} is running"
  else
    echo "Docker hosted registry ${reg_name} is NOT running"
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
  if [ "${reg_running}" == 'true' ]; then
    echo "Stopping docker hosted registry ${reg_name}"
    docker stop "${reg_name}"
    docker rm "${reg_name}"
  fi
  echo "Shutdown."
  exit 0
fi

if [ "${reg_running}" != 'true' ]; then
  docker run \
    -d --restart=always -p "${reg_port}:5000" --name "${reg_name}" \
    registry:2
fi

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --name ${cluster_name} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: ${node_image}
- role: worker
  image: ${node_image}
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_name}:${reg_port}"]
EOF

# connect the registry to the cluster network
if [ "${running}" != 'true' ]; then
  docker network connect "kind" "${reg_name}"
fi

# tell https://tilt.dev to use the registry
# https://docs.tilt.dev/choosing_clusters.html#discovering-the-registry
for node in $(kind get nodes); do
  kubectl annotate node "${node}" "kind.x-k8s.io/registry=localhost:${reg_port}";
done

echo "To use local registry:"
echo "export KO_DOCKER_REPO=localhost:${reg_port}"