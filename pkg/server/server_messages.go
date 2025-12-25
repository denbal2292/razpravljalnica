package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Node) PostMessage(ctx context.Context, req *pb.PostMessageRequest) (*pb.Message, error) {
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text cannot be empty")
	}
	if req.TopicId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "topic_id must be positive")
	}
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}

	// Send event to replication chain and wait for confirmation
	event := s.eventBuffer.CreateMessageEvent(req)
	s.logEventReceived(event)

	if err := s.replicateAndWaitForAck(context.Background(), event); err != nil {
		return nil, err
	}

	s.logApplyEvent(event)
	// We can now safely commit the message to storage with the event timestamp
	message, err := s.storage.PostMessage(req.TopicId, req.UserId, req.Text, event.EventAt)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return message, nil
}

func (s *Node) UpdateMessage(ctx context.Context, req *pb.UpdateMessageRequest) (*pb.Message, error) {
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

	// Send event to replication chain and wait for confirmation
	event := s.eventBuffer.UpdateMessageEvent(req)
	s.logEventReceived(event)

	if err := s.replicateAndWaitForAck(context.Background(), event); err != nil {
		return nil, err
	}

	// We can now safely commit the message update to storage
	s.logApplyEvent(event)
	message, err := s.storage.UpdateMessage(req.TopicId, req.UserId, req.MessageId, req.Text)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return message, nil
}

func (s *Node) DeleteMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*emptypb.Empty, error) {
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
	event := s.eventBuffer.DeleteMessageEvent(req)
	s.logEventReceived(event)
	if err := s.replicateAndWaitForAck(context.Background(), event); err != nil {
		return nil, err
	}

	s.logApplyEvent(event)
	// We can now safely commit the message deletion to storage
	err := s.storage.DeleteMessage(req.TopicId, req.UserId, req.MessageId)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Node) LikeMessage(ctx context.Context, req *pb.LikeMessageRequest) (*pb.Message, error) {
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
	event := s.eventBuffer.LikeMessageEvent(req)
	s.logEventReceived(event)

	if err := s.replicateAndWaitForAck(context.Background(), event); err != nil {
		return nil, err
	}

	s.logApplyEvent(event)
	// We can now safely commit the like to storage
	message, err := s.storage.LikeMessage(req.TopicId, req.UserId, req.MessageId)
	if err != nil {
		return nil, handleStorageError(err)
	}
	s.logApplyEvent(event)

	return message, nil
}

func (s *Node) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.GetMessagesResponse, error) {
	if req.TopicId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "topic_id must be positive")
	}
	if req.Limit <= 0 {
		return nil, status.Error(codes.InvalidArgument, "limit must be positive")
	}

	messages, err := s.storage.GetMessages(req.TopicId, req.FromMessageId, req.Limit)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return &pb.GetMessagesResponse{Messages: messages}, nil
}

func (s *Node) SubscribeTopic(req *pb.SubscribeTopicRequest, stream pb.MessageBoardSubscriptions_SubscribeTopicServer) error {
	// TODO: Implement topic subscription streaming logic
	return status.Error(codes.Unimplemented, "SubscribeTopic is not yet implemented")
}
