package server

import (
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Node struct {
	pb.UnimplementedMessageBoardReadsServer         // reads
	pb.UnimplementedMessageBoardWritesServer        // writes
	pb.UnimplementedMessageBoardSubscriptionsServer // subscriptions
	pb.UnimplementedChainReplicationServer
	storage     *storage.Storage
	eventBuffer *EventBuffer
	predecessor *NodeConnection // nil if HEAD
	successor   *NodeConnection // nil if TAIL
}

type NodeConnection struct {
	address string
	client  pb.ChainReplicationClient // gRPC client to the connected node
}

func NewServer(predecessor *NodeConnection, successor *NodeConnection) *Node {
	return &Node{
		storage:     storage.NewStorage(),
		eventBuffer: NewEventBuffer(),
		predecessor: predecessor,
		successor:   successor,
	}
}

func (s *Node) IsHead() bool {
	return s.predecessor == nil
}

func (s *Node) IsTail() bool {
	return s.successor == nil
}

// Apply the given event to the local storage
func (n *Node) applyEvent(event *pb.Event) error {
	if event.Like != nil {
		return n.storage.ApplyLikeEvent(event.Message, event.Like)
	} else if event.Message != nil {
		return n.storage.ApplyMessageEvent(event.Message, event.Op)
	} else if event.User != nil {
		return n.storage.ApplyUserEvent(event.User, event.Op)
	} else if event.Topic != nil {
		return n.storage.ApplyTopicEvent(event.Topic, event.Op)
	} else {
		panic("invalid event: no message, user, or topic")
	}
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
