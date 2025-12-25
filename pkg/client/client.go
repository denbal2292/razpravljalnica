package client

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ServerAddresses holds the addresses for HEAD and TAIL servers - this
// might be useful later when we add more complex functionality (subscriptions)
type ServerAddresses struct {
	AddrHead string
	AddrTail string
}

// ClientSet holds gRPC clients for different services
type ClientSet struct {
	Reads        pb.MessageBoardReadsClient
	Writes       pb.MessageBoardWritesClient
	Subsciptions pb.MessageBoardSubscriptionsClient
}

// Initialize a new client instance
func RunClient(serverUrls ServerAddresses) {
	fmt.Printf("Client connecting to head server at: %s\n", serverUrls.AddrHead)
	fmt.Printf("Client connecting to tail server at: %s\n", serverUrls.AddrTail)

	connHead, err := grpc.NewClient(
		serverUrls.AddrHead,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		panic(err)
	}
	defer connHead.Close()

	connTail, err := grpc.NewClient(
		serverUrls.AddrTail,
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
