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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/kelseyhightower/envconfig"
)

/*
Small function that demonstrates two concepts:

1. Deserializing / Serializing data
2. Using direct reply to modify incoming events (decorating in this example)

Incoming data is assumbed to be CloudEventBaseData with two fields
 Sequence int
 Message string

This function appends env[MESSAGE] to the Message portion of the data.
It is used by the samples in the Knative sequence section:
https://knative.dev/docs/eventing/sequence/
*/

type envConfig struct {
	DefaultResponseCode int `envconfig:"DEFAULT_RESPONSE_CODE" required:"true"`
}

var (
	env envConfig
)

// We just want a payload to result with.
type payload struct {
	ResponseCode int `json:"responsecode,omitempty"`
}

type failer struct {
	defaultResponseCode int
}

func NewFailer(defaultResponseCode int) *failer {
	return &failer{defaultResponseCode: defaultResponseCode}
}

func (f *failer) gotEvent(inputEvent event.Event) (*event.Event, error) {
	data := &payload{}
	rc := f.defaultResponseCode
	err := inputEvent.DataAs(data)
	if err != nil {
		log.Printf("Got error while unmarshalling data, using default response code: %s\n", err.Error())
	} else if data.ResponseCode != 0 {
		rc = data.ResponseCode
		log.Printf("using response code: %d\n", rc)
	}

	return nil, cloudevents.NewHTTPResult(rc, "Responding with: %d", rc)
}

func main() {
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s", err)
		os.Exit(1)
	}

	c, err := cloudevents.NewDefaultClient()
	if err != nil {
		log.Fatal("failed to create client, %v", err)
	}

	f := NewFailer(env.DefaultResponseCode)
	log.Printf("listening on 8080, default responsecode %d\n", env.DefaultResponseCode)
	log.Fatalf("failed to start receiver: %s\n", c.StartReceiver(context.Background(), f.gotEvent))
}
