package server

import (
	"fmt"

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
	ackSync     *AckSynchronization
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
		ackSync:     NewAckSynchronization(),
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
	switch event.Op {
	case pb.OpType_OP_POST:
		msgRequest := event.PostMessage
		_, err := n.storage.PostMessage(msgRequest.TopicId, msgRequest.UserId, msgRequest.Text, event.EventAt)
		return err

	case pb.OpType_OP_UPDATE:
		msgRequest := event.UpdateMessage
		_, err := n.storage.UpdateMessage(msgRequest.TopicId, msgRequest.UserId, msgRequest.MessageId, msgRequest.Text)
		return err

	case pb.OpType_OP_DELETE:
		msgRequest := event.DeleteMessage
		return n.storage.DeleteMessage(msgRequest.TopicId, msgRequest.UserId, msgRequest.MessageId)

	case pb.OpType_OP_LIKE:
		likeRequest := event.LikeMessage
		_, err := n.storage.LikeMessage(likeRequest.TopicId, likeRequest.UserId, likeRequest.MessageId)
		return err

	case pb.OpType_OP_CREATE_USER:
		userRequest := event.CreateUser
		_, err := n.storage.CreateUser(userRequest.Name)
		return err

	case pb.OpType_OP_CREATE_TOPIC:
		topicRequest := event.CreateTopic
		_, err := n.storage.CreateTopic(topicRequest.Name)
		return err

	default:
		panic(fmt.Sprintf("unknown event operation: %v", event.Op))
	}

}

// Convert storage layer errors to appropriate gRPC status codes.
// TODO: Expand (UserNotFound, TopicNotFound, etc.)
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
