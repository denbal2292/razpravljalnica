package server

import (
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Node struct {
	pb.UnimplementedMessageBoardServer
	pb.UnimplementedChainReplicationServer
	storage     *storage.Storage
	predecessor *NodeConnection // nil if HEAD
	successor   *NodeConnection // nil if TAIL
}

type NodeConnection struct {
	address string
	client  pb.MessageBoardClient // gRPC client to the connected node
}

func NewServer(predecessor *NodeConnection, successor *NodeConnection) *Node {
	return &Node{
		storage:     storage.NewStorage(),
		predecessor: predecessor,
		successor:   successor,
	}
}

func (s *Node) isHead() bool {
	return s.predecessor == nil
}

func (s *Node) isTail() bool {
	return s.successor == nil
}

// Convert storage layer errors to appropriate gRPC status codes.
func handleStorageError(err error) error {
	switch err {
	case storage.ErrTopicNotFound, storage.ErrUserNotFound, storage.ErrMsgNotFound:
		return status.Error(codes.NotFound, err.Error())
	case storage.ErrUserNotAuthor:
		return status.Error(codes.PermissionDenied, err.Error())
	case storage.ErrUserAlreadyLiked:
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
