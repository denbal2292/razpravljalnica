package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

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

	messages, err := n.storage.GetMessages(req.TopicId, req.FromMessageId, req.Limit)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return &pb.GetMessagesResponse{Messages: messages}, nil
}

func (n *Node) SubscribeTopic(req *pb.SubscribeTopicRequest, stream pb.MessageBoardSubscriptions_SubscribeTopicServer) error {
	// TODO: Implement topic subscription streaming logic
	return status.Error(codes.Unimplemented, "SubscribeTopic is not yet implemented")
}
