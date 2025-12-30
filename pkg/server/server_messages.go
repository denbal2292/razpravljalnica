package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// TODO: If this is not HEAD node, we should probably reject the request
func (n *Node) PostMessage(ctx context.Context, req *pb.PostMessageRequest) (*pb.Message, error) {
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text cannot be empty")
	}
	if req.TopicId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "topic_id must be positive")
	}
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}

	if n.IsSyncing() {
		n.logger.Info("Node is syncing, waiting for sync to complete before posting message")
		n.WaitForSyncToComplete()
	}

	// TODO: Don't apply it here in createEvent method!
	// Send event to replication chain and wait for confirmation
	event := n.eventBuffer.CreateMessageEvent(req)
	n.logEventReceived(event)

	if err := n.handleEventReplicationAndWaitForAck(event); err != nil {
		return nil, err
	}

	// We can now safely commit the message to storage with the event timestamp
	n.logApplyEvent(event)

	message, err := n.storage.PostMessage(req.TopicId, req.UserId, req.Text, event.EventAt)
	if err != nil {
		return nil, handleStorageError(err)
	}

	// Notify subscription manager about the new message event
	subEvent := n.subscriptionManager.CreateMessageEvent(message, event.SequenceNumber, event.EventAt, pb.OpType_OP_POST)
	n.subscriptionManager.AddMessageEvent(subEvent, req.TopicId)

	return message, nil
}

func (n *Node) UpdateMessage(ctx context.Context, req *pb.UpdateMessageRequest) (*pb.Message, error) {
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text cannot be empty")
	}
	if req.TopicId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "topic_id must be positive")
	}
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}
	if req.MessageId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "message_id must be positive")
	}

	if n.IsSyncing() {
		n.logger.Info("Node is syncing, waiting for sync to complete before updating message")
		n.WaitForSyncToComplete()
	}

	// Send event to replication chain and wait for confirmation
	event := n.eventBuffer.UpdateMessageEvent(req)
	n.logEventReceived(event)

	if err := n.handleEventReplicationAndWaitForAck(event); err != nil {
		return nil, err
	}

	// We can now safely commit the message update to storage
	n.logApplyEvent(event)

	message, err := n.storage.UpdateMessage(req.TopicId, req.UserId, req.MessageId, req.Text)
	if err != nil {
		return nil, handleStorageError(err)
	}

	// Notify subscription manager about the new message event
	subEvent := n.subscriptionManager.CreateMessageEvent(message, event.SequenceNumber, event.EventAt, pb.OpType_OP_UPDATE)
	n.subscriptionManager.AddMessageEvent(subEvent, req.TopicId)

	return message, nil
}

func (n *Node) DeleteMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*emptypb.Empty, error) {
	if req.TopicId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "topic_id must be positive")
	}
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}
	if req.MessageId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "message_id must be positive")
	}

	if n.IsSyncing() {
		n.logger.Info("Node is syncing, waiting for sync to complete before deleting message")
		n.WaitForSyncToComplete()
	}

	// Send event to replication chain and wait for confirmation
	event := n.eventBuffer.DeleteMessageEvent(req)
	n.logEventReceived(event)

	if err := n.handleEventReplicationAndWaitForAck(event); err != nil {
		return nil, err
	}

	// We can now safely commit the message deletion to storage
	n.logApplyEvent(event)

	message, err := n.storage.DeleteMessage(req.TopicId, req.UserId, req.MessageId)
	if err != nil {
		return nil, handleStorageError(err)
	}

	// Notify subscription manager about the new message event
	subEvent := n.subscriptionManager.CreateMessageEvent(message, event.SequenceNumber, event.EventAt, pb.OpType_OP_DELETE)
	n.subscriptionManager.AddMessageEvent(subEvent, req.TopicId)

	return &emptypb.Empty{}, nil
}

func (n *Node) LikeMessage(ctx context.Context, req *pb.LikeMessageRequest) (*pb.Message, error) {
	if req.TopicId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "topic_id must be positive")
	}
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}
	if req.MessageId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "message_id must be positive")
	}

	// Send event to replication chain and wait for confirmation
	event := n.eventBuffer.LikeMessageEvent(req)
	n.logEventReceived(event)

	if err := n.handleEventReplicationAndWaitForAck(event); err != nil {
		return nil, err
	}

	// We can now safely commit the like to storage
	n.logApplyEvent(event)

	message, err := n.storage.LikeMessage(req.TopicId, req.UserId, req.MessageId)
	if err != nil {
		return nil, handleStorageError(err)
	}

	// Notify subscription manager about the new message event
	subEvent := n.subscriptionManager.CreateMessageEvent(message, event.SequenceNumber, event.EventAt, pb.OpType_OP_LIKE)
	n.subscriptionManager.AddMessageEvent(subEvent, req.TopicId)

	return message, nil
}

func (n *Node) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.GetMessagesResponse, error) {
	if req.TopicId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "topic_id must be positive")
	}
	if req.Limit <= 0 {
		return nil, status.Error(codes.InvalidArgument, "limit must be positive")
	}

	messages, err := n.storage.GetMessagesWithLimit(req.TopicId, req.FromMessageId, req.Limit)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return &pb.GetMessagesResponse{Messages: messages}, nil
}

func (n *Node) SubscribeTopic(req *pb.SubscribeTopicRequest, stream grpc.ServerStreamingServer[pb.MessageEvent]) error {
	// Validate subscribe token
	if !n.subscriptionManager.ValidateSubscriptionToken(req.SubscribeToken, req.UserId) {
		return status.Error(codes.InvalidArgument, "invalid subscription token")
	}

	// 1. Start listening for message events for this subscriptions
	var eventChan <-chan *pb.MessageEvent

	// Ensure subscription is cleared when function exits
	defer n.subscriptionManager.ClearSubscription(req.SubscribeToken)

	// 2. Fetch past messages from storage
	pastMessages, err := n.storage.GetMessagesFromTopics(req.TopicId, req.FromMessageId, func() {
		// Open subscription channel in a callback while the storage lock is held
		// to make sure no messages are added while we are fetching past messages
		eventChan = n.subscriptionManager.OpenSubscriptionChannel(req)
	})

	if err != nil {
		return handleStorageError(err)
	}

	// 3. Send past messages first
	for _, msg := range pastMessages {
		subEvent := n.subscriptionManager.CreateMessageEvent(msg, 0, msg.CreatedAt, pb.OpType_OP_POST)
		if err := stream.Send(subEvent); err != nil {
			n.logger.Error("Failed to send past message event to subscriber", "error", err)
			return err
		}
	}

	// 4. Stream events to the client as they arrive
	for event := range eventChan {
		if err := stream.Send(event); err != nil {
			n.logger.Error("Failed to send message event to subscriber", "error", err)
			return err
		}
	}

	return nil
}
