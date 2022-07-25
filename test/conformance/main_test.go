//go:build e2e
// +build e2e

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

package rekt

import (
	"os"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"knative.dev/pkg/logging"

	_ "knative.dev/eventing/test/test_images"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/reconciler-test/pkg/environment"
)

// global is the singleton instance of GlobalEnvironment. It is used to parse
// the testing config for the test run. The config will specify the cluster
// config as well as the parsing level and state flags.
var global environment.GlobalEnvironment
var eventingGlobal EventingGlobal

type EventingGlobal struct {
	BrokerClass        string `envconfig:"BROKER_CLASS" default:"MTChannelBasedBroker" required:"true"`
	BrokerTemplatesDir string `envconfig:"BROKER_TEMPLATES"`
}

// TestMain is the first entry point for `go test`.
func TestMain(m *testing.M) {

	global = environment.NewStandardGlobalEnvironment(func(cfg environment.Configuration) environment.Configuration {
		// Increase the timeout for polling, rabbit brokers take a while to go
		// ready sometimes (due to PVC).
		cfg.Context = environment.ContextWithPollTimings(cfg.Context, time.Second*5, time.Minute*5)

		log := logging.FromContext(cfg.Context)
		// Process EventingGlobal.
		if err := envconfig.Process("", &eventingGlobal); err != nil {
			log.Fatal("Failed to process env var", zap.Error(err))
		}
		return cfg
	})

	// Run the tests.
	os.Exit(m.Run())
}
