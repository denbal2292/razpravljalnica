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
}

// TODO: Clean this up
func main() {
	// Parse command-line flags
	startPort := flag.Int("port", 9876, "The port for the first node in the chain")
	numNodes := flag.Int("nodes", 3, "Number of nodes in the chain")
	flag.Parse()

	if *numNodes < 1 {
		fmt.Println("Number of nodes must be at least 1")
		return
	}

	// Get hostname for logging
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	// Create n servers in a chain
	nodes := make([]*server.Node, *numNodes)
	gRPCServers := make([]*grpc.Server, *numNodes)
	listeners := make([]net.Listener, *numNodes)

	// Create servers and listeners
	for i := 0; i < *numNodes; i++ {
		nodes[i] = server.NewServer()
		// configure node identity and logger prefix
		// nodes[i].SetIdentity(i, *numNodes)
		gRPCServers[i] = grpc.NewServer()

		// Create TCP listener for each server on sequential ports
		url := fmt.Sprintf("%s:%d", "localhost", *startPort+i)
		listener, err := net.Listen("tcp", url)
		if err != nil {
			panic(err)
		}
		listeners[i] = listener
	}

	// Connect nodes in order with gRPC clients
	for i := 0; i < *numNodes; i++ {
		// Set predecessor (nil for head)
		if i == 0 {
			nodes[i].SetPredecessor(nil)
		} else {
			// Connect to predecessor's port
			predecessorAddr := fmt.Sprintf("localhost:%d", *startPort+i-1)
			conn, err := grpc.NewClient(predecessorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				panic(fmt.Sprintf("Failed to connect to predecessor at %s: %v", predecessorAddr, err))
			}

			nodes[i].SetPredecessor(&server.NodeConnection{
				Client: razpravljalnica.NewChainReplicationClient(conn),
			})
		}

		// Set successor (nil for tail)
		if i == *numNodes-1 {
			nodes[i].SetSuccessor(nil)
		} else {
			// Connect to successor's port
			successorAddr := fmt.Sprintf("localhost:%d", *startPort+i+1)
			conn, err := grpc.NewClient(successorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				panic(fmt.Sprintf("Failed to connect to successor at %s: %v", successorAddr, err))
			}

			nodes[i].SetSuccessor(&server.NodeConnection{
				Client: razpravljalnica.NewChainReplicationClient(conn),
			})
		}
	}

	// Register services for all nodes
	for i, node := range nodes {
		registerServices(node, gRPCServers[i])
	}

	fmt.Printf("Chain replication with %d servers:\n", *numNodes)
	for i := 0; i < *numNodes; i++ {
		role := "Middle"
		switch i {
		case 0:
			role = "Head"
		case *numNodes - 1:
			role = "Tail"
		}
		fmt.Printf("  Server %d (%s): localhost:%d\n", i, role, *startPort+i)
	}
	fmt.Printf("Servers running on %s (%s)\n", hostname, hostname)

	// Start all gRPC servers in goroutines (except the last one)
	for i := 0; i < *numNodes-1; i++ {
		go func(idx int) {
			if err := gRPCServers[idx].Serve(listeners[idx]); err != nil {
				panic(err)
			}
		}(i)
	}

	// Start the last server in the main goroutine
	if err := gRPCServers[*numNodes-1].Serve(listeners[*numNodes-1]); err != nil {
		panic(err)
	}
}
