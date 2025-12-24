package client

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Initialize a new client instance
func RunClient(url string) {
	fmt.Printf("gRPC client connecting to URL %s\n", url)
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials((insecure.NewCredentials())))

	if err != nil {
		panic(err)
	}
	defer conn.Close()

	grpcClient := pb.NewMessageBoardClient(conn)

	// Simple REPL loop
	// Useful for proper line reading
	scanner := bufio.NewScanner(os.Stdin)
	for {
		// Read the command
		fmt.Print(">: ")
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

		if err != nil {
			panic(err)
		}

		if err := route(grpcClient, command, args); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}
