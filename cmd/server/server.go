package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	razpravljalnica "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/server"
	"google.golang.org/grpc"
)

func main() {
	// Parse command-line flags
	port := flag.Int("port", 9876, "The server port")
	flag.Parse()

	// Construct server URL
	url := fmt.Sprintf("%s:%d", "localhost", *port)

	// Create gRPC server
	gRPCServer := grpc.NewServer()

	// Create and register MessageBoard server
	messageBoardServer := server.NewServer(nil, nil)
	razpravljalnica.RegisterMessageBoardServer(gRPCServer, messageBoardServer)

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	// Create TCP listener
	listener, err := net.Listen("tcp", url)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Server running on %s (%s)\n", url, hostname)

	// Start gRPC server
	if err := gRPCServer.Serve(listener); err != nil {
		panic(err)
	}

}
