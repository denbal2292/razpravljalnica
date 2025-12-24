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
	node := server.NewServer(nil, nil)

	// Register services based on the node position
	// Only head nodes handle writes
	if node.IsHead() {
		razpravljalnica.RegisterMessageBoardWritesServer(gRPCServer, node)
	}

	// Only tail nodes handle reads
	if node.IsTail() {
		razpravljalnica.RegisterMessageBoardReadsServer(gRPCServer, node)
	}

	// All nodes handle subscriptions
	razpravljalnica.RegisterChainReplicationServer(gRPCServer, node)

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
