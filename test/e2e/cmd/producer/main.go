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
	"context"
	"log"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	_ "knative.dev/pkg/system/testing"
)

type envConfig struct {
	Sink  string `envconfig:"K_SINK" required:"true"`
	Count int    `envconfig:"COUNT" default:"10"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s", err)
		os.Exit(1)
	}
	ctx := cloudevents.ContextWithTarget(context.Background(), env.Sink)

	p, err := cloudevents.NewHTTP()
	if err != nil {
		log.Fatalf("failed to create protocol: %s", err.Error())
	}

	c, err := cloudevents.NewClient(p, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	send(cloudevents.ContextWithRetriesExponentialBackoff(ctx, 10*time.Millisecond, 10), c, env.Count)

	// Wait.
	<-ctx.Done()
}

func send(ctx context.Context, c cloudevents.Client, count int) {
	for i := 0; i < count; i++ {
		e := cloudevents.NewEvent()
		e.SetType("knative.producer.e2etest")
		e.SetSource("https://knative.dev/eventing/e2e")
		e.SetExtension("index", i)
		_ = e.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
			"id":      i,
			"message": "Hello, World!",
		})

		if result := c.Send(ctx, e); cloudevents.IsUndelivered(result) {
			log.Printf("Failed to send: %s", result.Error())
		} else if cloudevents.IsACK(result) {
			log.Printf("Sent: %d", i)
		} else if cloudevents.IsNACK(result) {
			log.Printf("Sent but not accepted: %s", result.Error())
		}
		time.Sleep(50 * time.Millisecond)
	}
}
