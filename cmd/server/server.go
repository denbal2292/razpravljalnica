package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	razpravljalnica "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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
}

func main() {
	port := flag.Int("port", 9876, "Port to listen on")
	// prev := flag.String("prev", "", "Address of predecessor node (empty if head)")
	// next := flag.String("next", "", "Address of successor node (empty if tail)")
	flag.Parse()

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	controlPlaneAddress := "localhost:50051" // TODO: Make configurable

	conn, err := grpc.NewClient(
		controlPlaneAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	controlPlaneClient := razpravljalnica.NewControlPlaneClient(conn)

	node := server.NewServer("server-"+hostname, fmt.Sprintf("localhost:%d", *port), controlPlaneClient)
	gRPCServer := grpc.NewServer()
	razpravljalnica.RegisterNodeUpdateServer(gRPCServer, node)

	addr := fmt.Sprintf("localhost:%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	fmt.Println("Starting node:")
	fmt.Println("  Address:", addr)
	fmt.Println("  Host:", hostname)
	// fmt.Println("  Role:",
	// 	func() string {
	// 		switch {
	// 		case *prev == "" && *next == "":
	// 			return "Single (Head & Tail)"
	// 		case *prev == "":
	// 			return "Head"
	// 		case *next == "":
	// 			return "Tail"
	// 		default:
	// 			return "Middle"
	// 		}
	// 	}(),
	// )

	// Connect to neighbors AFTER startup
	// connectToNeighbors(node, *prev, *next)
	registerServices(node, gRPCServer)

	// Start serving
	if err := gRPCServer.Serve(listener); err != nil {
		panic(err)
	}
}
