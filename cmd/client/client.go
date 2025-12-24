package main

import (
	"flag"
	"fmt"

	"github.com/denbal2292/razpravljalnica/pkg/client"
)

func main() {
	sPtr := flag.String("s", "localhost", "server URL")
	pPtr := flag.Int("p", 9876, "server port")
	flag.Parse()

	url := fmt.Sprintf("%s:%d", *sPtr, *pPtr)

	// Run the client
	client.RunClient(url)
}
