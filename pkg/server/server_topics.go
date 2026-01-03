package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) CreateTopic(ctx context.Context, req *pb.CreateTopicRequest) (*pb.Topic, error) {
	// Writes are allowed only on HEAD
	if err := n.requireHead(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	// Send event to replication chain and wait for confirmation
	event := n.eventBuffer.CreateTopicEvent(req)
	n.logEventReceived(event)

	result := n.handleEventReplicationAndWaitForAck(event)
	n.logApplyEvent(event)

	if result.err != nil {
		return nil, handleStorageError(result.err)
	}

	topic := result.topic

	return topic, nil
}

func (n *Node) ListTopics(ctx context.Context, req *emptypb.Empty) (*pb.ListTopicsResponse, error) {
	// Reads are allowed only on TAIL
	if err := n.requireTail(); err != nil {
		return nil, err
	}

	topics, err := n.storage.ListTopics()
	if err != nil {
		return nil, handleStorageError(err)
	}

	return &pb.ListTopicsResponse{Topics: topics}, nil
}
