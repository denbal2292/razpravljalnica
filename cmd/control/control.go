package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/denbal2292/razpravljalnica/pkg/control"
	razpravljalnica "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
)

func main() {
	// Parse command-line flags
	port := flag.Int("port", 50051, "The server port")
	flag.Parse()

	// Construct server URL
	url := fmt.Sprintf("%s:%d", "localhost", *port)

	// Create gRPC server
	gRPCServer := grpc.NewServer()

	// Create and register MessageBoard server
	control := control.NewControlPlane()

	razpravljalnica.RegisterClientDiscoveryServer(gRPCServer, control)
	razpravljalnica.RegisterControlPlaneServer(gRPCServer, control)

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	// Create TCP listener
	listener, err := net.Listen("tcp", url)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Control plane running on %s (%s)\n", url, hostname)

	// Start gRPC server
	if err := gRPCServer.Serve(listener); err != nil {
		panic(err)
	}
}
