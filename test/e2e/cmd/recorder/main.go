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
	"flag"
	"os"

	"knative.dev/pkg/injection"

	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing/pkg/test/observer"
	recorder_vent "knative.dev/eventing/pkg/test/observer/recorder-vent"
	writer_vent "knative.dev/eventing/pkg/test/observer/writer-vent"
)

func main() {
	flag.Parse()
	ctx, _ := injection.EnableInjectionOrDie(nil, nil) //nolint

	obs := observer.New(
		writer_vent.NewEventLog(ctx, os.Stdout),
		recorder_vent.NewFromEnv(ctx),
	)

	if err := obs.Start(ctx); err != nil {
		panic(err)
	}
}
