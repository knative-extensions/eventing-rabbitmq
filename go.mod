module knative.dev/eventing-rabbitmq

go 1.14

require (
	github.com/NeowayLabs/wabbit v0.0.0-20200409220312-12e68ab5b0c6
	github.com/aws/aws-sdk-go v1.34.11 // indirect
	github.com/cloudevents/sdk-go/v2 v2.2.0
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c
	github.com/docker/go-connections v0.4.0
	github.com/fsouza/go-dockerclient v1.6.5 // indirect
	github.com/go-openapi/spec v0.19.7 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/go-cmp v0.5.2
	github.com/google/licenseclassifier v0.0.0-20200708223521-3d09a0ea2f39
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/influxdata/tdigest v0.0.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/michaelklishin/rabbit-hole/v2 v2.4.0
	github.com/n3wscott/rigging v0.3.0
	github.com/onsi/ginkgo v1.14.0 // indirect
	github.com/streadway/amqp v1.0.0
	github.com/testcontainers/testcontainers-go v0.7.0
	github.com/tiago4orion/conjure v0.0.0-20150908101743-93cb30b9d218 // indirect
	go.uber.org/zap v1.16.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.18.8
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	knative.dev/eventing v0.19.1-0.20201117061051-47ee6e3586ca
	knative.dev/hack v0.0.0-20201112185459-01a34c573bd8
	knative.dev/pkg v0.0.0-20201117020252-ab1a398f669c
)

replace (
	// This branch allows creation of headers exchange.
	github.com/NeowayLabs/wabbit => github.com/vaikas/wabbit v0.0.0-20201002085521-b5b22698ecc7

	// Grab the latest so we get modifiable retries
	github.com/cloudevents/sdk-go/v2 => github.com/cloudevents/sdk-go/v2 v2.3.1-0.20201008104108-58f826d67d91
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190717161051-705d9623b7c1
	// Somehow mattmoor/bindings causes grief...
	github.com/google/go-github/v32 => github.com/google/go-github/v32 v32.0.1-0.20200624231906-3d244d3d496e

	// WORKAROUND until KEDA v2 is not released
	github.com/operator-framework/operator-sdk => github.com/operator-framework/operator-sdk v0.11.0

	// lock prom import to avoid a bad goautoneg import.
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/apiserver => k8s.io/apiserver v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.8
)
