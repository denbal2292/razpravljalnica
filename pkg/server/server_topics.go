package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) CreateTopic(ctx context.Context, req *pb.CreateTopicRequest) (*pb.Topic, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	// Send event to replication chain and wait for confirmation
	event := n.eventBuffer.CreateTopicEvent(req)
	n.logEventReceived(event)

	if err := n.replicateAndWaitForAck(event); err != nil {
		return nil, err
	}

	n.logApplyEvent(event)

	topic, err := n.storage.CreateTopic(req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return topic, nil
}

func (n *Node) ListTopics(ctx context.Context, req *emptypb.Empty) (*pb.ListTopicsResponse, error) {
	topics, err := n.storage.ListTopics()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.ListTopicsResponse{Topics: topics}, nil
}
