package client

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

func listTopics(grpcClient pb.MessageBoardReadsClient, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	topics, err := grpcClient.ListTopics(ctx, &emptypb.Empty{})

	if err != nil {
		return fmt.Errorf("failed to list topics: %w", err)
	}

	if len(topics.Topics) == 0 {
		fmt.Println("No topics yet.")
		return nil
	}

	fmt.Println("Topics:")
	for _, topic := range topics.Topics {
		fmt.Println(topic.Id, topic.Name)
	}

	return nil
}

func getUser(grpcClient pb.MessageBoardReadsClient, args []string) error {
	if err := requireArgs(args, 1, "user <user_id>"); err != nil {
		return err
	}

	userId, err := parseId(args[0], "user_id")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	user, err := grpcClient.GetUser(ctx, &pb.GetUserRequest{
		UserId: userId,
	})

	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	fmt.Println(user.Id, user.Name)

	return nil
}

func getMessages(grpcClient pb.MessageBoardReadsClient, args []string) error {
	if err := requireArgs(args, 3, "messages <topic_id> <from_id> <limit_id>"); err != nil {
		return err
	}

	topicId, err := parseId(args[0], "topic_id")
	if err != nil {
		return err
	}

	fromId, err := parseId(args[1], "from_id")
	if err != nil {
		return err
	}

	limit, err := parseInt32(args[2], "limit")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	messagesResp, err := grpcClient.GetMessages(ctx, &pb.GetMessagesRequest{
		TopicId:       topicId,
		FromMessageId: fromId,
		Limit:         int32(limit),
	})

	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	for _, message := range messagesResp.Messages {
		fmt.Println(message.Id, message.UserId, message.TopicId, message.Text)
	}

	return nil
}
