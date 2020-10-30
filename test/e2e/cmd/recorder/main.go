/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"knative.dev/eventing/test/test_images"
	"log"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"

	"knative.dev/pkg/injection"

	_ "knative.dev/pkg/system/testing"

	loggerVent "knative.dev/eventing/test/lib/recordevents/logger_vent"
	"knative.dev/eventing/test/lib/recordevents/observer"
	recorderVent "knative.dev/eventing/test/lib/recordevents/recorder_vent"
)

func main() {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Error while reading the cfg", err)
	}
	//nolint // nil ctx is fine here, look at the code of EnableInjectionOrDie
	ctx, _ := injection.EnableInjectionOrDie(nil, cfg)
	ctx = test_images.ConfigureLogging(ctx, "recordevents")

	obs := observer.NewFromEnv(ctx,
		loggerVent.Logger(logging.FromContext(ctx).Infof),
		recorderVent.NewFromEnv(ctx),
	)

	if err := obs.Start(ctx); err != nil {
		panic(err)
	}
}
