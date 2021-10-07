While https://github.com/knative-sandbox/eventing-rabbitmq/pull/445 contains the YAML that configures the benchmark, and we (+@ikvmw) also intend to have the step-by-step guide in that PR as well, I wanted to start with the WIP guide variant here, and move it across when it's done. Here it goes:

## 1/5. Pre-requisites
- GKE 1.20 with autoscaling and `c2-standard-4` machine type in the default nodepool
- kubectl 1.20
- [ko 0.9.3](https://github.com/google/ko/releases/tag/v0.9.3) + [`KO_DOCKER_REPO`](https://github.com/knative/eventing/blob/main/DEVELOPMENT.md#setup-your-environment)
- A git clone of [knative-sandbox/eventing-rabbitmq](https://github.com/knative-sandbox/eventing-rabbitmq) at tag v0.27.0 or higher
We use [GCP `c2-standard-4`](https://cloud.google.com/compute/vm-instance-pricing#compute-optimized_machine_types)  machine type to make benchmarks results consistent. Using a different instance type, or a different cloud provider, is almost certain to yield different results.
## 2/5. Install Knative Serving
We install a specific version of Knative - [v0.26.0](https://github.com/knative/eventing/releases/tag/v0.26.0) - as this is a point-in-time guide. While we expect subsequent versions to continue working the same way, in the absence of automated tests that ensure this, we stick to exact versions that we have tested manually.
```
# Installing Knative Serving ...
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.26.0/serving-crds.yaml
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.26.0/serving-core.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.26.0/eventing-crds.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.26.0/eventing-core.yaml

# Installing Knative Serving Kourier networking layer ...
kubectl apply --filename https://github.com/knative/net-kourier/releases/download/v0.26.0/kourier.yaml
# Patching Knative Serving to use Kourier for the networking layer ...
kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress.class":"kourier.ingress.networking.knative.dev"}}'
```
> Aside: does this look right to you @benmoss ? https://github.com/knative/serving/releases/download/v0.26.1/serving-crds.yaml . I didn't install v0.26.1 - latest release - because of it.
## 3/5. Install Knative Eventing RabbitMQ
```
# Installing RabbitMQ Operator ...
kubectl apply --filename https://github.com/rabbitmq/cluster-operator/releases/download/v1.9.0/cluster-operator.yml
# Installing cert-manager ...
curl -sL https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml \
| sed 's/kube-system/cert-manager/' \
| kubectl apply --namespace cert-manager --filename -
# Installing RabbitMQ Messaging Topology Operator with cert-manager integration ...
kubectl apply --filename https://github.com/rabbitmq/messaging-topology-operator/releases/download/v1.2.1/messaging-topology-operator-with-certmanager.yaml
# Installing Knative Eventing RabbitMQ Broker ...
kubectl apply --filename https://github.com/knative-sandbox/eventing-rabbitmq/releases/download/v0.26.0/rabbitmq-broker.yaml
```
In case you want to test the latest dev version of eventing-rabbitmq, follow these steps:
```
# ⚠️ TODO after https://github.com/knative-sandbox/eventing-rabbitmq/pull/445 is merged:
# git clone https://github.com/knative-sandbox/eventing-rabbitmq $GOPATH/src/knative.dev/eventing-rabbitmq
git clone https://github.com/ikvmw/eventing-rabbitmq $GOPATH/src/knative.dev/eventing-rabbitmq
cd $GOPATH/src/knative.dev/eventing-rabbitmq
git checkout -b issue-239 origin/issue-239
ko apply --filename config/broker
```
## 4/5. Run Knative Eventing RabbitMQ benchmarks
```
cd $GOPATH/src/knative.dev/eventing-rabbitmq
ko apply --filename test/performance/broker-rmq/100-broker-perf-setup.yaml
kubectl wait --for=condition=AllReplicasReady=true rmq/rabbitmq-test-cluster --timeout=10m --namespace perf-eventing
kubectl wait --for=condition=IngressReady=true brokers/rabbitmq-test-broker --timeout=10m --namespace perf-eventing
kubectl wait --for=condition=SubscriberResolved=true triggers/rabbitmq-broker-perf --timeout=10m --namespace perf-eventing
ko apply --filename test/performance/broker-rmq/200-broker-perf.yaml
```
## 5/5. Download & visualise Knative Eventing RabbitMQ benchmark results
Pre-requisite: [gnuplot](http://www.gnuplot.info/) (on macOS it's `brew install gnuplot`)
```
kubectl wait --for=condition=complete job/rabbitmq-broker-perf-send-receive --timeout=10m --namespace perf-eventing
kubectl port-forward rabbitmq-broker-perf-aggregator 10001:10001 --namespace perf-eventing &
PORT_FORWARD_PID=$!
cd $GOPATH/src/knative.dev/eventing-rabbitmq/test/performance
curl http://localhost:10001/results > eventing-rabbitmq-broker-perf-results.csv
kill $PORT_FORWARD_PID
wait $PORT_FORWARD_PID 2>/dev/null
gnuplot -c latency-and-throughput.plg eventing-rabbitmq-broker-perf-results.csv 0.5 0 1100
```
![image](https://user-images.githubusercontent.com/90614196/136284992-b1ab58bd-17f5-4f01-bdb8-14655edad86e.png)
To visualise just the end-to-end event latency, run: `gnuplot -c latency.plg eventing-rabbitmq-broker-perf-results.csv 0.5 0 1100`
![image](https://user-images.githubusercontent.com/90614196/136285323-caa5dd6b-34fa-439e-9f7a-de4a9fd021ef.png)
To visualise just the end-to-end event throughput, run: `gnuplot -c throughput.plg eventing-rabbitmq-broker-perf-results.csv 0.5 0 1100`
![image](https://user-images.githubusercontent.com/90614196/136285415-676e8647-5a70-414c-a135-c9cf7795a103.png)
> Notice in the screenshot above that the receiver througput line is around the zero axis, which makes me believe that we are just sending messages, but not receiving them. We should double-check.
So what do the `0.5 0 1100` arguments mean?
- `0.5` is the time in seconds, and it is the max allowed size for the y1 axis
- `0` and `1100` are the message throughput, and it they represent the min and max boundaries of the y2 axis
