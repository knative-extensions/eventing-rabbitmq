module github.com/n3wscott/rigging

go 1.14

require (
	github.com/google/go-cmp v0.5.2
	github.com/kelseyhightower/envconfig v1.4.0
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/klog v1.0.0
	knative.dev/pkg v0.0.0-20201012163217-54ad6c6d39a7
)

replace k8s.io/client-go => k8s.io/client-go v0.18.8
