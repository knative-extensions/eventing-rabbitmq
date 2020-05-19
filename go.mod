module knative.dev/eventing-rabbitmq

go 1.13

require (
	github.com/cloudevents/sdk-go v1.2.0
	github.com/cloudevents/sdk-go/v2 v2.0.0-RC4
	github.com/google/ko v0.5.1 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/sbcd90/wabbit v0.0.0-20190419210920-43bc2261e0e0
	github.com/streadway/amqp v0.0.0-20190404075320-75d898a42a94
	go.uber.org/zap v1.14.1
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	knative.dev/eventing v0.14.2
	knative.dev/pkg v0.0.0-20200507220045-66f1d63f1019
	knative.dev/serving v0.14.1-0.20200507214552-b5ed1dd92906 // indirect
	knative.dev/test-infra v0.0.0-20200507205145-ddb008b1759b
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/apiserver => k8s.io/apiserver v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.4
)
