module knative.dev/eventing-rabbitmq

go 1.14

require (
	github.com/cloudevents/sdk-go v1.2.0 // indirect
	github.com/cloudevents/sdk-go/v2 v2.1.0
	github.com/docker/cli v0.0.0-20200303162255-7d407207c304 // indirect
	github.com/go-yaml/yaml v2.1.0+incompatible // indirect
	github.com/golang/lint v0.0.0-20180702182130-06c8688daad7 // indirect
	github.com/google/go-cmp v0.4.0
	github.com/google/ko v0.5.0 // indirect
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/klauspost/cpuid v1.2.2 // indirect
	github.com/knative/build v0.1.2 // indirect
	github.com/moby/moby v1.13.1 // indirect
	github.com/nats-io/gnatsd v1.4.1 // indirect
	github.com/nats-io/go-nats v1.7.0 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/sbcd90/wabbit v0.0.0-20190419210920-43bc2261e0e0
	github.com/shurcooL/go v0.0.0-20180423040247-9e1955d9fb6e // indirect
	github.com/streadway/amqp v0.0.0-20190404075320-75d898a42a94
	go.uber.org/zap v1.14.1
	gotest.tools/v3 v3.0.2 // indirect
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.1
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/kubernetes v1.14.7 // indirect
	knative.dev/eventing v0.16.1-0.20200712014307-acdd118f5ff0
	knative.dev/pkg v0.0.0-20200702222342-ea4d6e985ba0
	knative.dev/serving v0.16.0 // indirect
	knative.dev/test-infra v0.0.0-20200630141629-15f40fe97047
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/apiserver => k8s.io/apiserver v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
