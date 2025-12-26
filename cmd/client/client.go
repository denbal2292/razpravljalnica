package main

import (
	"flag"

	"github.com/denbal2292/razpravljalnica/pkg/client"
)

func main() {
	urlControlPlane := flag.String(
		"control",
		"localhost:9000",
		"control plane URL",
	)

	flag.Parse()

	// Run the client
	client.RunClient(*urlControlPlane)
}
