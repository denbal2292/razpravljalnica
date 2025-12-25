package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Node) CreateTopic(ctx context.Context, req *pb.CreateTopicRequest) (*pb.Topic, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	// Send event to replication chain and wait for confirmation
	event := s.eventBuffer.CreateTopicEvent(req)
	s.logEventReceived(event)

	if err := s.replicateAndWaitForAck(context.Background(), event); err != nil {
		return nil, err
	}

	// We can now safely commit the topic to storage
	s.logApplyEvent(event)
	topic, err := s.storage.CreateTopic(req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return topic, nil
}

func (s *Node) ListTopics(ctx context.Context, req *emptypb.Empty) (*pb.ListTopicsResponse, error) {
	topics, err := s.storage.ListTopics()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.ListTopicsResponse{Topics: topics}, nil
}
