module knative.dev/eventing-rabbitmq

go 1.14

require (
	github.com/NeowayLabs/wabbit v0.0.0-20200409220312-12e68ab5b0c6 // indirect
	github.com/cloudevents/sdk-go/v2 v2.2.0
	github.com/docker/go-connections v0.4.0
	github.com/fsouza/go-dockerclient v1.6.5 // indirect
	github.com/google/go-cmp v0.5.1
	github.com/google/licenseclassifier v0.0.0-20200708223521-3d09a0ea2f39
	github.com/kedacore/keda v1.4.2-0.20200617120630-97df7e08e24b
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/michaelklishin/rabbit-hole v1.5.0
	github.com/n3wscott/rigging v0.0.1
	github.com/sbcd90/wabbit v0.0.0-20190419210920-43bc2261e0e0
	github.com/streadway/amqp v0.0.0-20200108173154-1c71cc93ed71
	github.com/testcontainers/testcontainers-go v0.7.0
	github.com/tiago4orion/conjure v0.0.0-20150908101743-93cb30b9d218 // indirect
	go.uber.org/zap v1.15.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.18.8
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	knative.dev/eventing v0.17.1-0.20200909104450-6781aa6c20d9
	knative.dev/pkg v0.0.0-20200908235250-56fba14ba7df
	knative.dev/test-infra v0.0.0-20200908182932-5a8105609141
	knative.dev/eventing v0.17.1-0.20200903132832-43a1bf784bae
	knative.dev/eventing-autoscaler-keda v0.0.0-20200827145907-65c254c7a70f
	knative.dev/pkg v0.0.0-20200902221531-b0307fc6d285
	knative.dev/test-infra v0.0.0-20200831235415-fac473dda98b
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
