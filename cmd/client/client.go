package main

import (
	"flag"

	"github.com/denbal2292/razpravljalnica/pkg/client"
)

func main() {
	// Server HEAD address and port
	urlHead := flag.String("head", "localhost:9000", "head server URL")

	// Server TAIL address and port
	urlTail := flag.String("tail", "localhost:9002", "tail server URL")

	flag.Parse()

	// Run the client
	client.RunClient(client.ServerAddresses{
		AddrHead: *urlHead,
		AddrTail: *urlTail,
	})
}
