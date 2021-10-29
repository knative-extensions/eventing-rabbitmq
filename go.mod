module knative.dev/eventing-rabbitmq

go 1.16

require (
	github.com/NeowayLabs/wabbit v0.0.0-20210927194032-73ad61d1620e
	github.com/cloudevents/sdk-go/v2 v2.4.1
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/go-connections v0.4.0
	github.com/fsouza/go-dockerclient v1.6.5 // indirect
	github.com/go-openapi/spec v0.19.7 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/go-cmp v0.5.6
	github.com/google/licenseclassifier v0.0.0-20200708223521-3d09a0ea2f39
	github.com/influxdata/tdigest v0.0.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/michaelklishin/rabbit-hole/v2 v2.10.0
	github.com/pkg/errors v0.9.1
	github.com/rabbitmq/amqp091-go v0.0.0-20210921101955-bb8191b6c914
	github.com/rabbitmq/cluster-operator v1.8.2
	github.com/rabbitmq/messaging-topology-operator v0.11.0
	github.com/testcontainers/testcontainers-go v0.7.0
	github.com/tiago4orion/conjure v0.0.0-20150908101743-93cb30b9d218 // indirect
	go.uber.org/zap v1.19.1
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.21.4
	k8s.io/apiextensions-apiserver v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/code-generator v0.21.4
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	knative.dev/eventing v0.26.1-0.20211029085753-34c08a406eca
	knative.dev/hack v0.0.0-20211028194650-b96d65a5ff5e
	knative.dev/pkg v0.0.0-20211028235650-5d9d300c2e40
	knative.dev/reconciler-test v0.0.0-20211029073051-cff9b538d33c
	sigs.k8s.io/controller-runtime v0.9.6
)

replace github.com/docker/docker => github.com/docker/engine v0.0.0-20190717161051-705d9623b7c1
