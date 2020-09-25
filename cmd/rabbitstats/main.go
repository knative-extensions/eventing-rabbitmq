package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
)

const rabbitURL = "amqp://-MKSJPZPXIx6IyVMpCFCUcgf586NvvCU:NBfTI_zUti1ibescDakDWdgK-2f09eIr@rokn-rabbitmq-client.default:15672"

func handler(w http.ResponseWriter, r *http.Request) {
	log.Print("helloworld: received a request")
	target := os.Getenv("TARGET")
	if target == "" {
		target = "World"
	}
	fmt.Fprintf(w, "Hello %s!\n", target)

	uri, err := url.Parse(rabbitURL)
	if err != nil {
		fmt.Printf("failed to parse Broker URL: %v", err)
	}
	host, _, err := net.SplitHostPort(uri.Host)
	adminURL := fmt.Sprintf("http://%s:%d", host, 15672)
	p, _ := uri.User.Password()
	c, err := rabbithole.NewClient(adminURL, uri.User.Username(), p)
	if err != nil {
		fmt.Printf("Failed to connect to rabbit: %s\n", err)
	}
	xs, err := c.ListExchanges()
	if err != nil {
		fmt.Printf("Failed to list exchanges: %s\n", err)
	}
	for i := range xs {
		fmt.Printf("Exchange:\n%+v\n", xs[i])
	}
	qs, err := c.ListQueues()
	if err != nil {
		fmt.Printf("Failed to list queues: %s\n", err)
	}
	for i := range qs {
		fmt.Printf("Queue:\n%+v\n", qs[i])
	}
	bs, err := c.ListBindings()
	if err != nil {
		fmt.Printf("Failed to list bindings: %s\n", err)
	}
	for i := range bs {
		fmt.Printf("Binding:\n%+v\n", bs[i])
	}
}

func main() {
	log.Print("helloworld: starting server...")

	http.HandleFunc("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("helloworld: listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
