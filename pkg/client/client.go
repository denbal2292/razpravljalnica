package client

import (
	"context"
	"fmt"

	"github.com/denbal2292/razpravljalnica/pkg/client/cli"
	"github.com/denbal2292/razpravljalnica/pkg/client/gui"
	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Initialize a new client instance
func RunClient(controlPlaneAddrs []string, clientType string) {
	clients, err := newClientSet(controlPlaneAddrs)
	if err != nil {
		fmt.Println("Failed to create client:", err)
		clients.Close()
		return
	}
	defer clients.Close()

	fmt.Println("Successfully connected to the server.")

	// Start CLI client
	switch clientType {
	case "cli":
		cli.StartCLIClient(clients)
	case "gui":
		gui.StartGUIClient(clients)
	default:
		fmt.Println("Unknown client type:", clientType)
	}
}

// newClientSet incrementally builds the client set
func newClientSet(controlPlaneAddrs []string) (*shared.ClientSet, error) {
	// Establish control plane connection
	clients := &shared.ClientSet{
		ControlPlaneAddrs: controlPlaneAddrs,
	}

	// Get HEAD and TAIL addresses using the retry mechanism
	var headAddr, tailAddr string
	err := clients.TryControlPlaneRequest(func(client pb.ClientDiscoveryClient) error {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		serverConns, err := client.GetClusterState(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}

		if serverConns.Head == nil || serverConns.Tail == nil {
			return fmt.Errorf("no nodes available in the cluster")
		}

		headAddr = serverConns.Head.Address
		tailAddr = serverConns.Tail.Address
		return nil
	})

	if err != nil {
		return clients, err
	}

	// Establish HEAD connection
	connHead, err := grpc.NewClient(
		headAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return clients, err
	}
	clients.HeadConn = connHead

	// Establish TAIL connection
	connTail, err := grpc.NewClient(
		tailAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return clients, err
	}
	clients.TailConn = connTail

	// Create gRPC clients for all services
	clients.Writes = pb.NewMessageBoardWritesClient(connHead)
	clients.Reads = pb.NewMessageBoardReadsClient(connTail)
	// clients.Subscriptions = pb.NewMessageBoardSubscriptionsClient(connHead)

	return clients, nil
}
