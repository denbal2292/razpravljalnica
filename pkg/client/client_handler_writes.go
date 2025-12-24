package client

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

func createUser(grpcClient pb.MessageBoardWritesClient, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("createuser requires 1 argument: <name>")
	}

	username := strings.Join(args, " ")

	fmt.Printf("Creating user with name: %s\n", username)

	// Set a timeout for the request
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// Potentially the operation can completel before timeout - we release
	// resources associated with the context
	defer cancel()

	user, err := grpcClient.CreateUser(ctx, &pb.CreateUserRequest{
		Name: username,
	})

	if err != nil {
		// Use %w format specifier to wrap the error - err can be checked
		// with errors.Is and errors.As
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Println(user.Id, user.Name)

	return nil
}

func createTopic(grpcClient pb.MessageBoardWritesClient, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("createtopic requires 1 argument: <name>")
	}

	topicName := strings.Join(args, " ")

	fmt.Printf("Creating topic with name: %s\n", topicName)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	topic, err := grpcClient.CreateTopic(ctx, &pb.CreateTopicRequest{
		Name: topicName,
	})

	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	fmt.Println(topic.Id, topic.Name)

	return nil
}

func postMessage(grpcClient pb.MessageBoardWritesClient, args []string) error {
	return nil
}

func updateMessage(grpcClient pb.MessageBoardWritesClient, args []string) error {
	return nil
}

func deleteMessage(grpcClient pb.MessageBoardWritesClient, args []string) error {
	return nil
}

func likeMessage(grpcClient pb.MessageBoardWritesClient, args []string) error {
	return nil
}
