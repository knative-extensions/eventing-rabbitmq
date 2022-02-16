/*
Copyright 2022 The Knative Authors

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
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var (
	delay int
)

func init() {
	flag.IntVar(&delay, "delay", 20, "delay in seconds when processing messages")
}

// display prints the given Event's ID.
func display(event cloudevents.Event) {
	time.Sleep(time.Duration(delay) * time.Second)
	fmt.Printf("processed event with ID: %s\n", event.ID())
}

func main() {
	run(context.Background())
}

func run(ctx context.Context) {
	c, err := client.NewClientHTTP(nil, nil)
	if err != nil {
		log.Fatal("Failed to create client: ", err)
	}

	if err := c.StartReceiver(ctx, display); err != nil {
		log.Fatal("Error during receiver's runtime: ", err)
	}
	log.Print("starting event display\n")
}
