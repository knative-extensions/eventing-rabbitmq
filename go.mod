module knative.dev/eventing-rabbitmq

go 1.14

require (
	github.com/NeowayLabs/wabbit v0.0.0-20200409220312-12e68ab5b0c6 // indirect
	github.com/cloudevents/sdk-go/v2 v2.1.0
	github.com/docker/go-connections v0.4.0
	github.com/fsouza/go-dockerclient v1.6.5 // indirect
	github.com/google/go-cmp v0.4.0
	github.com/grpc-ecosystem/grpc-gateway v1.12.2 // indirect
	github.com/influxdata/tdigest v0.0.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/michaelklishin/rabbit-hole v1.5.0
	github.com/n3wscott/rigging v0.0.1
	github.com/sbcd90/wabbit v0.0.0-20190419210920-43bc2261e0e0
	github.com/streadway/amqp v0.0.0-20190404075320-75d898a42a94
	github.com/testcontainers/testcontainers-go v0.7.0
	github.com/tiago4orion/conjure v0.0.0-20150908101743-93cb30b9d218 // indirect
	go.uber.org/zap v1.14.1
	google.golang.org/grpc v1.28.1 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.1
	k8s.io/client-go v12.0.0+incompatible
	knative.dev/eventing v0.16.1
	knative.dev/pkg v0.0.0-20200702222342-ea4d6e985ba0
	knative.dev/test-infra v0.0.0-20200630141629-15f40fe97047
)

replace (
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190717161051-705d9623b7c1
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/apiserver => k8s.io/apiserver v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
