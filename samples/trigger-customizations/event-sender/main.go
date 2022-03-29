/*
Copyright 2022 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

var (
	sink       string
	inputEvent string
	numMsg     int
)

func init() {
	flag.StringVar(&sink, "sink", "", "The sink url for the message destination.")
	flag.StringVar(&inputEvent, "event", "", "Event JSON encoded")
	flag.IntVar(&numMsg, "num-messages", 100, "The number of messages to attempt to send.")
}

func main() {
	flag.Parse()
	ctx := context.Background()
	ctx = cloudevents.WithEncodingBinary(ctx)

	httpOpts := []cehttp.Option{
		cloudevents.WithTarget(sink),
	}

	t, err := cloudevents.NewHTTP(httpOpts...)
	if err != nil {
		log.Fatalf("failed to create transport, %v", err)
	}

	c, err := cloudevents.NewClient(t)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	var baseEvent cloudevents.Event
	if err := json.Unmarshal([]byte(inputEvent), &baseEvent); err != nil {
		log.Fatalf("Unable to unmarshal the event from json: %v", err)
	}

	for i := 0; i < numMsg; i++ {
		event := baseEvent.Clone()
		event.SetID(fmt.Sprintf("%d", i+1))

		log.Printf("I'm going to send\n%s\n", event)

		if result := c.Send(ctx, event); !cloudevents.IsACK(result) {
			log.Printf("failed to send cloudevent: %s", result.Error())
		}
	}
}
