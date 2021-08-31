module knative.dev/eventing-rabbitmq

go 1.16

require (
	github.com/NeowayLabs/wabbit v0.0.0-20200409220312-12e68ab5b0c6
	github.com/cloudevents/sdk-go/v2 v2.4.1
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/go-connections v0.4.0
	github.com/fsouza/go-dockerclient v1.6.5 // indirect
	github.com/go-openapi/spec v0.19.7 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/go-cmp v0.5.6
	github.com/google/licenseclassifier v0.0.0-20200708223521-3d09a0ea2f39
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/influxdata/tdigest v0.0.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/michaelklishin/rabbit-hole/v2 v2.8.0
	github.com/pkg/errors v0.9.1
	github.com/rabbitmq/cluster-operator v1.6.0
	github.com/rabbitmq/messaging-topology-operator v0.8.0
	github.com/streadway/amqp v1.0.0
	github.com/testcontainers/testcontainers-go v0.7.0
	github.com/tiago4orion/conjure v0.0.0-20150908101743-93cb30b9d218 // indirect
	go.uber.org/zap v1.19.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.21.4
	k8s.io/apiextensions-apiserver v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/code-generator v0.21.4
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	knative.dev/eventing v0.25.1-0.20210830155228-9b1f09cb571c
	knative.dev/hack v0.0.0-20210806075220-815cd312d65c
	knative.dev/pkg v0.0.0-20210830224055-82f3a9f1c5bc
	knative.dev/reconciler-test v0.0.0-20210820180205-a25de6a08087
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	// This branch allows creation of headers exchange.
	github.com/NeowayLabs/wabbit => github.com/vaikas/wabbit v0.0.0-20201002085521-b5b22698ecc7

	github.com/docker/docker => github.com/docker/engine v0.0.0-20190717161051-705d9623b7c1

	// lock prom import to avoid a bad goautoneg import.
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
)
