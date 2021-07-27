package utils

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

func NamePrefixFilterFunc(prefix string) func(interface{}) bool {
	return func(obj interface{}) bool {
		if mo, ok := obj.(metav1.Object); ok {
			return strings.HasPrefix(mo.GetName(), prefix)
		}
		return false
	}
}

func SystemConfigMapsFilterFunc() func(interface{}) bool {
	return reconciler.ChainFilterFuncs(NamePrefixFilterFunc("config-"), reconciler.NamespaceFilterFunc(system.Namespace()))
}
