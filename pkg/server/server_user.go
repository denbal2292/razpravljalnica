package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Node) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	// Send event to replication chain and wait for confirmation
	event := s.eventBuffer.CreateUserEvent(req)
	s.logEventReceived(event)

	if err := s.replicateAndWaitForAck(context.Background(), event); err != nil {
		return nil, err
	}

	s.logApplyEvent(event)
	// We can now safely commit the user to storage
	user, err := s.storage.CreateUser(req.Name)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return user, nil
}

// Retrieve user by their ID
func (s *Node) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}

	user, err := s.storage.GetUser(req.UserId)
	if err != nil {
		return nil, handleStorageError(err)
	}

	return user, nil
}
