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
	clientType := flag.String(
		"type",
		"cli",
		"Type of client to run (cli or gui)",
	)
	flag.Parse()

	// In a Raft cluster, we have multiple control plane servers
	controlPlaneAddrs := []string{
		fmt.Sprintf("127.0.0.1:%d", *controlPlanePort),
		fmt.Sprintf("127.0.0.1:%d", *controlPlanePort+1),
		fmt.Sprintf("127.0.0.1:%d", *controlPlanePort+2),
	}

	// Run the client
	client.RunClient(controlPlaneAddrs, *clientType)
}
