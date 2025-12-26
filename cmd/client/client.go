package main

import (
	"flag"
	"fmt"

	"github.com/denbal2292/razpravljalnica/pkg/client"
)

func main() {
	controlPlanePort := flag.Int(
		"control-port",
		50051,
		"control plane port",
	)

	flag.Parse()
	controlPlaneAddress := fmt.Sprintf("localhost:%d", *controlPlanePort)

	// Run the client
	client.RunClient(controlPlaneAddress)
}
