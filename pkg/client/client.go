package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ServerAddresses holds the addresses for HEAD and TAIL servers - this
// might be useful later when we add more complex functionality (subscriptions)
// ClientSet holds gRPC clients for different services
type clientSet struct {
	// Connections
	controlConn *grpc.ClientConn
	headConn    *grpc.ClientConn
	tailConn    *grpc.ClientConn

	// Clients
	reads         pb.MessageBoardReadsClient
	writes        pb.MessageBoardWritesClient
	subscriptions pb.MessageBoardSubscriptionsClient
}

// Initialize a new client instance
func RunClient(controlPlaneAddress string) {
	fmt.Println("Connecting to control plane at", controlPlaneAddress)

	clients, err := newClientSet(controlPlaneAddress)
	defer clients.close()

	if err != nil {
		fmt.Println("Failed to create client:", err)
		return
	}

	// Simple REPL loop
	// Useful for proper line reading
	scanner := bufio.NewScanner(os.Stdin)
	for {
		// Read the command
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		// Trim leading and trailing space and ignore empty input
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Split the input into fields
		fields := strings.Fields(input)
		command := fields[0]
		args := fields[1:]

		if err := route(clients, command, args); err != nil {
			if errors.Is(err, ErrExit) {
				// Client decided to exit
				break
			}
			fmt.Printf("Error: %v\n", err)
		}
	}
}

// newClientSet incrementally builds the client set
func newClientSet(controlPlaneUrl string) (*clientSet, error) {
	// Establish control plane connection
	clients := &clientSet{}

	connControlPlane, err := grpc.NewClient(
		controlPlaneUrl,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return clients, err
	}
	clients.controlConn = connControlPlane

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
	clients.headConn = connHead

	// Establish TAIL connection
	connTail, err := grpc.NewClient(
		tailAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return clients, err
	}
	clients.tailConn = connTail

	// Create gRPC clients for all services
	clients.writes = pb.NewMessageBoardWritesClient(connHead)
	clients.reads = pb.NewMessageBoardReadsClient(connTail)
	clients.subscriptions = pb.NewMessageBoardSubscriptionsClient(connHead)

	return clients, nil
}

func (c *clientSet) close() {
	if c.controlConn != nil {
		c.controlConn.Close()
	}
	if c.headConn != nil {
		c.headConn.Close()
	}
	if c.tailConn != nil {
		c.tailConn.Close()
	}
}

// This might be useful later as a helper function upon server failure
func getHeadAndTailAddresses(controlPlaneConn *grpc.ClientConn) (headAddr, tailAddr string, err error) {
	controlPlaneClient := pb.NewClientDiscoveryClient(controlPlaneConn)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Get addresses of HEAD and TAIL servers from the control plane
	serverConns, err := controlPlaneClient.GetClusterState(
		ctx, &emptypb.Empty{},
	)

	if err != nil {
		return "", "", err
	}

	// Get the HEAD and TAIL addresses
	headAddr = serverConns.Head.Address
	tailAddr = serverConns.Tail.Address

	return headAddr, tailAddr, nil
}
