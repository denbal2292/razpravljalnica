package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func createUser(clients *shared.ClientSet, args []string) error {
	if err := requireArgs(args, 1, "createuser <name>"); err != nil {
		return err
	}

	username := strings.Join(args, " ")
	// Set a timeout for the request
	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	// Potentially the operation can completel before timeout - we release
	// resources associated with the context
	defer cancel()

	_, err := shared.RetryFetch(ctx, clients, func(ctx context.Context) (*pb.User, error) {
		return clients.Writes.CreateUser(ctx, &pb.CreateUserRequest{
			Name: username,
		})
	})

	if err != nil {
		// Use %w format specifier to wrap the error - err can be checked
		// with errors.Is and errors.As
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func createTopic(clients *shared.ClientSet, args []string) error {
	if err := requireArgs(args, 1, "createtopic <name>"); err != nil {
		return err
	}

	topicName := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	_, err := shared.RetryFetch(ctx, clients, func(ctx context.Context) (*pb.Topic, error) {
		return clients.Writes.CreateTopic(ctx, &pb.CreateTopicRequest{
			Name: topicName,
		})
	})

	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	return nil
}

func postMessage(clients *shared.ClientSet, args []string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	_, err = shared.RetryFetch(ctx, clients, func(ctx context.Context) (*pb.Message, error) {
		return clients.Writes.PostMessage(ctx, &pb.PostMessageRequest{
			UserId:  userId,
			TopicId: topicId,
			Text:    content,
		})
	})

	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	return nil
}

func updateMessage(clients *shared.ClientSet, args []string) error {
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
	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	_, err = shared.RetryFetch(ctx, clients, func(ctx context.Context) (*pb.Message, error) {
		return clients.Writes.UpdateMessage(ctx, &pb.UpdateMessageRequest{
			TopicId:   topicId,
			UserId:    userId,
			MessageId: msgId,
			Text:      text,
		})
	})

	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	return nil
}

func deleteMessage(clients *shared.ClientSet, args []string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	_, err = shared.RetryFetch(ctx, clients, func(ctx context.Context) (*emptypb.Empty, error) {
		return clients.Writes.DeleteMessage(ctx, &pb.DeleteMessageRequest{
			TopicId:   topicId,
			UserId:    userId,
			MessageId: msgId,
		})
	})

	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

func likeMessage(clients *shared.ClientSet, args []string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	_, err = shared.RetryFetch(ctx, clients, func(ctx context.Context) (*pb.Message, error) {
		return clients.Writes.LikeMessage(ctx, &pb.LikeMessageRequest{
			TopicId:   topicId,
			MessageId: msgId,
			UserId:    userId,
		})
	})

	if err != nil {
		return fmt.Errorf("failed to like message: %w", err)
	}

	return nil
}
