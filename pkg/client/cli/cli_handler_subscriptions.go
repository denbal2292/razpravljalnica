package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func subscribeTopics(grpcClient *grpc.ClientConn, args []string) error {
	if err := requireArgs(args, 2, "subscribe <user_id> <topic_id>..."); err != nil {
		return err
	}

	// Get the user ID
	userId, err := parseInt64(args[0])
	if err != nil {
		return err
	}

	// Get the topic IDs to subscribe to
	topicIds := make([]int64, len(args)-1)
	for i, arg := range args[1:] {
		topicId, err := parseInt64(arg)
		if err != nil {
			return err
		}
		topicIds[i] = topicId
	}

	// Start with the subscription process
	controlPlaneClient := pb.NewClientDiscoveryClient(grpcClient)

	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	// Request subscription node info
	subResponse, err := controlPlaneClient.GetSubscriptionNode(ctx, &pb.SubscriptionNodeRequest{
		UserId: userId,
	})

	if err != nil {
		return err
	}

	// Connect to the subscription node
	conn, err := grpc.NewClient(
		subResponse.Node.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Create a subscription client
	subClient := pb.NewMessageBoardSubscriptionsClient(conn)
	// Start from the beginning - for easier debugging
	fromMessageId := int64(0)

	subscriptionStream, err := subClient.SubscribeTopic(context.Background(), &pb.SubscribeTopicRequest{
		TopicId:        topicIds,
		UserId:         userId,
		FromMessageId:  fromMessageId,
		SubscribeToken: subResponse.SubscribeToken,
	})

	if err != nil {
		panic(err)
	}

	// Used to terminate a subscription on Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	err = handleSubscriptionStream(subscriptionStream, sigCh)

	return err
}

func handleSubscriptionStream(stream pb.MessageBoardSubscriptions_SubscribeTopicClient, sigCh <-chan os.Signal) error {
	// Read from the message stream
	fmt.Println("Listening for message events... Press Ctrl+C to stop.")
	for {
		msgCh := make(chan *pb.MessageEvent, 1)
		errCh := make(chan error, 1)
		go func() {
			msgEvent, err := stream.Recv()
			if err != nil {
				errCh <- err
				return
			}
			msgCh <- msgEvent
		}()

		// Avoid busy loop
		select {
		case <-sigCh:
			fmt.Println("\nSubscription interrupted by user.")
			return nil
		case msgEvent := <-msgCh:
			msg := msgEvent.Message
			fmt.Printf(
				"Received message event: Op=%v, TopicID=%d, MessageID=%d, UserID=%d, Content=%s\n", msgEvent.Op, msg.TopicId, msg.Id, msg.UserId, msg.Text,
			)
		case err := <-errCh:
			fmt.Printf("Stream error: %v\n", err)
			return err
		}
	}
}
