package main

import (
	"flag"
	"fmt"
	"net"

	razpravljalnica "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/server"
	"google.golang.org/grpc"
)

// TODO: Move this so we check the node role inside the node
func registerServices(node *server.Node, gRPCServer *grpc.Server) {
	// Only head nodes handle writes
	// if node.IsHead() {
	razpravljalnica.RegisterMessageBoardWritesServer(gRPCServer, node)
	// }

	// Only tail nodes handle reads
	// if node.IsTail() {
	razpravljalnica.RegisterMessageBoardReadsServer(gRPCServer, node)
	// }

	// All nodes participate in chain replication
	razpravljalnica.RegisterChainReplicationServer(gRPCServer, node)

	razpravljalnica.RegisterMessageBoardSubscriptionsServer(gRPCServer, node)
}

func main() {
	// Port 0 means to pick a random available port
	port := flag.Int("port", 0, "Port to listen on")
	controlPlanePort := flag.Int("control-port", 50051, "Control plane port")

	flag.Parse()

	// Build list of all control plane server addresses
	// In a Raft cluster, we have multiple control plane servers
	controlPlaneAddrs := []string{
		fmt.Sprintf("localhost:%d", *controlPlanePort),
		fmt.Sprintf("localhost:%d", *controlPlanePort+1),
		fmt.Sprintf("localhost:%d", *controlPlanePort+2),
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	addr := lis.Addr().String()

	node := server.NewServer("server-"+addr, addr, controlPlaneAddrs)
	gRPCServer := grpc.NewServer()
	razpravljalnica.RegisterNodeUpdateServer(gRPCServer, node)

	fmt.Println("Starting node:")
	fmt.Println("  Addr: ", addr)
	fmt.Println("  Control Plane Addrs:", controlPlaneAddrs)

	registerServices(node, gRPCServer)

	// Start serving
	if err := gRPCServer.Serve(lis); err != nil {
		panic(err)
	}
}
