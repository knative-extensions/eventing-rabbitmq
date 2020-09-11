module knative.dev/eventing-rabbitmq

go 1.14

require (
	github.com/NeowayLabs/wabbit v0.0.0-20200409220312-12e68ab5b0c6 // indirect
	github.com/cloudevents/sdk-go/v2 v2.2.0
	github.com/docker/go-connections v0.4.0
	github.com/fsouza/go-dockerclient v1.6.5 // indirect
	github.com/google/go-cmp v0.5.2
	github.com/google/licenseclassifier v0.0.0-20200708223521-3d09a0ea2f39
	github.com/kedacore/keda v1.5.1-0.20200909143349-82a4f899f7f1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/michaelklishin/rabbit-hole v1.5.0
	github.com/n3wscott/rigging v0.0.2-0.20200909204211-040bdb39a369
	github.com/sbcd90/wabbit v0.0.0-20190419210920-43bc2261e0e0
	github.com/streadway/amqp v1.0.0
	github.com/testcontainers/testcontainers-go v0.7.0
	github.com/tiago4orion/conjure v0.0.0-20150908101743-93cb30b9d218 // indirect
	go.uber.org/zap v1.15.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.18.8
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	knative.dev/eventing v0.17.1-0.20200908151032-5fdaa0605d87
	knative.dev/eventing-autoscaler-keda v0.0.0-20200909130950-f1b6899ad87b
	knative.dev/pkg v0.0.0-20200911145400-2d4efecc6bc1
	knative.dev/test-infra v0.0.0-20200909211651-72eb6ae3c773
)

replace (
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

	// WORKAROUND until k8s v1.18+ gets to knative/pkg and knative/eventing
	knative.dev/eventing => github.com/zroubalik/eventing v0.15.1-0.20200824120738-2b97ca8b85d0
)
