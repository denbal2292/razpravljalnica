package main

import (
	"flag"
	"fmt"

	"github.com/denbal2292/razpravljalnica/pkg/client"
)

func main() {
	// Server HEAD address
	shPtr := flag.String("sh", "localhost", "head server URL")
	// Server HEAD port
	phPtr := flag.Int("ph", 9876, "head server port")

	// Server TAIL address
	stPtr := flag.String("st", "localhost", "tail server URL")
	// Server TAIL port
	ptPtr := flag.Int("pt", 9877, "tail server port")

	flag.Parse()

	// URLs for head and TAIL servers
	urlHead := fmt.Sprintf("%s:%d", *shPtr, *phPtr)
	urlTail := fmt.Sprintf("%s:%d", *stPtr, *ptPtr)

	// Run the client
	client.RunClient(client.ServerAddresses{
		AddrHead: urlHead,
		AddrTail: urlTail,
	})
}
