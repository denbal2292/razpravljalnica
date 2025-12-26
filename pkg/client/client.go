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
type ClientSet struct {
	Reads        pb.MessageBoardReadsClient
	Writes       pb.MessageBoardWritesClient
	Subsciptions pb.MessageBoardSubscriptionsClient
}

// Initialize a new client instance
func RunClient(controlPlaneUrl string) {
	fmt.Println("Connecting to control plane at", controlPlaneUrl)

	// Connect to control plane to get HEAD and TAIL addresses
	connControlPlane, err := grpc.NewClient(
		controlPlaneUrl,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(err)
	}
	// We might need control plane connection in the client for requests in the
	// future
	defer connControlPlane.Close()

	// Initialize control plane client
	controlPlaneClient := pb.NewClientDiscoveryClient(connControlPlane)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Get addresses of HEAD and TAIL servers from the control plane
	serverConns, err := controlPlaneClient.GetClusterState(
		ctx, &emptypb.Empty{},
	)

	if err != nil {
		panic(err)
	}

	// Get the head and tail addresses
	headAddr := serverConns.Head.Address
	tailAddr := serverConns.Tail.Address

	// Connect to HEAD and TAIL servers
	connHead, err := grpc.NewClient(
		headAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(err)
	}
	defer connHead.Close()

	connTail, err := grpc.NewClient(
		tailAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(err)
	}
	defer connTail.Close()

	// Create gRPC clients for all services
	clients := &ClientSet{
		Reads:  pb.NewMessageBoardReadsClient(connTail),
		Writes: pb.NewMessageBoardWritesClient(connHead),
		// This isn't yet implemented
		Subsciptions: pb.NewMessageBoardSubscriptionsClient(connHead),
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
