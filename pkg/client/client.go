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
func RunClient(controlPlaneAddress, clientType string) {
	fmt.Println("Connecting to control plane at", controlPlaneAddress)

	clients, err := newClientSet(controlPlaneAddress)
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
func newClientSet(controlPlaneUrl string) (*shared.ClientSet, error) {
	// Establish control plane connection
	clients := &shared.ClientSet{}

	connControlPlane, err := grpc.NewClient(
		controlPlaneUrl,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return clients, err
	}
	clients.ControlConn = connControlPlane

	// Get HEAD and TAIL addresses
	headAddr, tailAddr, err := getHeadAndTailAddresses(connControlPlane)
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

// This might be useful later as a helper function upon server failure
func getHeadAndTailAddresses(controlPlaneConn *grpc.ClientConn) (headAddr, tailAddr string, err error) {
	controlPlaneClient := pb.NewClientDiscoveryClient(controlPlaneConn)

	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	// Get addresses of HEAD and TAIL servers from the control plane
	serverConns, err := controlPlaneClient.GetClusterState(
		ctx, &emptypb.Empty{},
	)

	if err != nil {
		return "", "", err
	}

	if serverConns.Head == nil || serverConns.Tail == nil {
		return "", "", fmt.Errorf("No nodes available in the cluster")
	}

	// Get the HEAD and TAIL addresses
	headAddr = serverConns.Head.Address
	tailAddr = serverConns.Tail.Address

	return headAddr, tailAddr, nil
}
