package client

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

func createUser(grpcClient pb.MessageBoardWritesClient, args []string) error {
	if err := requireArgs(args, 1, "createuser <name>"); err != nil {
		return err
	}

	username := strings.Join(args, " ")
	// Set a timeout for the request
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// Potentially the operation can completel before timeout - we release
	// resources associated with the context
	defer cancel()

	_, err := grpcClient.CreateUser(ctx, &pb.CreateUserRequest{
		Name: username,
	})

	if err != nil {
		// Use %w format specifier to wrap the error - err can be checked
		// with errors.Is and errors.As
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func createTopic(grpcClient pb.MessageBoardWritesClient, args []string) error {
	if err := requireArgs(args, 1, "createtopic <name>"); err != nil {
		return err
	}

	topicName := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := grpcClient.CreateTopic(ctx, &pb.CreateTopicRequest{
		Name: topicName,
	})

	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	return nil
}

func postMessage(grpcClient pb.MessageBoardWritesClient, args []string) error {
	if err := requireArgs(args, 3, "post <user_id> <topic_id> <content>"); err != nil {
		return err
	}

	userId, err := parseId(args[0], "user_id")
	if err != nil {
		return err
	}

	topicId, err := parseId(args[1], "topic_id")
	if err != nil {
		return err
	}

	content := strings.Join(args[2:], " ")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = grpcClient.PostMessage(ctx, &pb.PostMessageRequest{
		UserId:  userId,
		TopicId: topicId,
		Text:    content,
	})

	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	return nil
}

func updateMessage(grpcClient pb.MessageBoardWritesClient, args []string) error {
	if err := requireArgs(args, 4, "update <topic_id> <user_id> <msg_id> <text>"); err != nil {
		return err
	}

	topicId, err := parseId(args[0], "topic_id")
	if err != nil {
		return err
	}

	userId, err := parseId(args[1], "user_id")
	if err != nil {
		return err
	}

	msgId, err := parseId(args[2], "msg_id")
	if err != nil {
		return err
	}

	text := strings.Join(args[3:], " ")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = grpcClient.UpdateMessage(ctx, &pb.UpdateMessageRequest{
		TopicId:   topicId,
		UserId:    userId,
		MessageId: msgId,
		Text:      text,
	})

	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	return nil
}

func deleteMessage(grpcClient pb.MessageBoardWritesClient, args []string) error {
	if err := requireArgs(args, 3, "delete <topic_id> <user_id> <msg_id>"); err != nil {
		return err
	}

	topicId, err := parseId(args[0], "topic_id")
	if err != nil {
		return err
	}

	userId, err := parseId(args[1], "user_id")
	if err != nil {
		return err
	}

	msgId, err := parseId(args[2], "msg_id")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = grpcClient.DeleteMessage(ctx, &pb.DeleteMessageRequest{
		TopicId:   topicId,
		UserId:    userId,
		MessageId: msgId,
	})

	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

func likeMessage(grpcClient pb.MessageBoardWritesClient, args []string) error {
	if err := requireArgs(args, 3, "like <topic_id> <msg_id> <user_id>"); err != nil {
		return err
	}

	topicId, err := parseId(args[0], "topic_id")
	if err != nil {
		return err
	}

	msgId, err := parseId(args[1], "msg_id")
	if err != nil {
		return err
	}

	userId, err := parseId(args[2], "user_id")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = grpcClient.LikeMessage(ctx, &pb.LikeMessageRequest{
		TopicId:   topicId,
		MessageId: msgId,
		UserId:    userId,
	})

	if err != nil {
		return fmt.Errorf("failed to like message: %w", err)
	}

	return nil
}
