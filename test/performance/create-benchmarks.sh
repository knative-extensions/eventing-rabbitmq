#!/usr/bin/env bash

# Copyright 2022 The Knative Authors
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

aggregator_port_forwarding() { # args: entity
  kubectl port-forward rabbitmq-$1-perf-aggregator 10001:10001 --namespace perf-eventing &
  sleep 5
}

wait_for_performance_job() { # args: entity
  kubectl wait --for=condition=complete job/rabbitmq-$1-perf-send-receive --timeout=10m --namespace perf-eventing
  sleep 10 # sometimes the perf work takes some extra time to finish
}

run_tests() { # args: test_type entity yaml_number graph_y_upper_limit
  echo "Starting $1 tests\n"
  envsubst < $script_path/$2-setup/$3-$2-$1-setup.yaml | ko apply -f -
  sleep 5
  echo "Running the $1 tests\n"
  wait_for_performance_job $2
  aggregator_port_forwarding $2
  echo "Finished.\nGenerating $1 tests results graphs\n"
  generate_result_graphics $2 $4 $1
  echo "Cleaning $1 perf resources\n"
  pkill -f "port-forward"
  kubectl delete -f $script_path/$2-setup/$3-$2-$1-setup.yaml
  sleep 5
}

generate_result_graphics() { # args: entity graph_y_upper_limit test_type
  curl http://localhost:10001/results > $script_path/eventing-rabbitmq-$1-perf-results.csv
  for type in latency-throughput latency throughput
  do
    gnuplot -c $script_path/$type.plg $script_path/eventing-rabbitmq-$1-perf-results.csv 0.8 0 $2 $results_path/$1/$3/parallel-$PARALLELISM-$type.png
  done
  rm $script_path/eventing-rabbitmq-$1-perf-results.csv
}

cleanup() { # args: entity
  kubectl delete -f $script_path/$1-setup/
  while kubectl get ns | grep perf-eventing
  do
    sleep 4
  done
}

if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
  echo "To generate the results for a release, run:\n./create-benchmarks.sh release-vx.y"
  exit
elif [ -z "$1" ]; then
  echo "Please pass a release version (release-vx.y) or -h/--help"
  exit
fi

RELEASE_VERSION=$1

if [ -z "$KO_DOCKER_REPO" ]; then
  echo "Please set KO_DOCKER_REPO env variable before running this script"
  exit
fi

echo "Generating resources for $RELEASE_VERSION"
echo "Cleaning up and creating results dirs for the broker\n"
script_path=$(dirname "$0")
results_path=$script_path/results/$RELEASE_VERSION

rm -rf $results_path
mkdir $results_path $results_path/broker $results_path/broker/constant-load $results_path/broker/increasing-load $results_path/broker/multi-consumer
for i in 1 100
do
  echo "Starting generating RabbitMQ's Broker results..."
  echo "Starting the Broker's setup with Trigger's parallelism: $i\n"
  export PARALLELISM=$i
  envsubst < $script_path/broker-setup/100-broker-perf-setup.yaml | ko apply -f -
  kubectl wait --for=condition=AllReplicasReady=true rmq/rabbitmq-test-cluster --timeout=10m --namespace perf-eventing
  kubectl wait --for=condition=IngressReady=true brokers/rabbitmq-test-broker --timeout=10m --namespace perf-eventing
  kubectl wait --for=condition=SubscriberResolved=true triggers/rabbitmq-trigger-perf --timeout=10m --namespace perf-eventing

  run_tests increasing-load broker 200 1100
  run_tests constant-load broker 300 1100
  if [ $i -gt 1 ]; then
    run_tests multi-consumer broker 400 2000
  fi
  echo "Cleaning broker perf tests resources\n"
  cleanup broker
done

mkdir $results_path/source $results_path/source/constant-load $results_path/source/increasing-load
for i in 1 100
do
  echo "Starting generating RabbitMQ's Source results..."
  echo "Starting the Source's setup with Trigger's parallelism: $i\n"
  export PARALLELISM=$i
  kubectl apply -f $script_path/source-setup/100-rabbitmq-setup.yaml
  kubectl wait --for=condition=AllReplicasReady=true rmq/rabbitmq-test-cluster --timeout=10m --namespace perf-eventing
  kubectl wait --for=condition=IngressReady=true brokers/rabbitmq-test-broker --timeout=10m --namespace perf-eventing
  sleep 10 # for some reason without this sleep the next steps fail most of the time, suspecting about RMQ reconciling delays
  export EXCHANGE_NAME=$(kubectl get exchanges -n perf-eventing -o jsonpath={.items[0].metadata.name})
  echo "Binding Ingress Exchange $EXCHANGE_NAME to RabbitMQ's Source\n"
  envsubst < $script_path/source-setup/200-source-perf-setup.yaml | kubectl apply -f -

  run_tests increasing-load source 300 1100
  run_tests constant-load source 400 1100
  echo "Cleaning source perf tests resources\n"
  cleanup source
done

echo "Finished building the performance results"
