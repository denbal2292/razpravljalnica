package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (n *Node) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
	// Writes are allowed only on HEAD
	if err := n.requireHead(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	// Send event to replication chain and wait for confirmation
	event := n.eventBuffer.CreateUserEvent(req)
	n.logEventReceived(event)

	result := n.handleEventReplicationAndWaitForAck(event)
	n.logApplyEvent(event)

	if result.err != nil {
		return nil, handleStorageError(result.err)
	}

	user := result.user
	return user, nil
}

// Retrieve user by their ID
func (n *Node) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	// Reads are allowed only on TAIL
	if err := n.requireTail(); err != nil {
		return nil, err
	}

	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}

	user, err := n.storage.GetUser(req.UserId)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return user, nil
}
