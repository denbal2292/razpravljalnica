package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lmittmann/tint"
)

type Node struct {
	pb.UnimplementedMessageBoardReadsServer         // reads
	pb.UnimplementedMessageBoardWritesServer        // writes
	pb.UnimplementedMessageBoardSubscriptionsServer // subscriptions
	pb.UnimplementedChainReplicationServer          // for communication between nodes in the chain
	pb.UnimplementedNodeUpdateServer                // for control plane to notify about neighbor changes

	nodeInfo          *pb.NodeInfo          // name and address of this node
	controlPlane      pb.ControlPlaneClient // gRPC client to the control plane
	heartbeatInterval time.Duration         // interval between heartbeats

	storage     *storage.Storage
	eventBuffer *EventBuffer

	predecessor *NodeConnection // nil if HEAD
	successor   *NodeConnection // nil if TAIL

	logger *slog.Logger // logger for the node
}

type NodeConnection struct {
	// address string
	Client pb.ChainReplicationClient // gRPC client to the connected node
}

func NewServer(name string, address string, controlPlane pb.ControlPlaneClient) *Node {
	n := &Node{
		storage:     storage.NewStorage(),
		eventBuffer: NewEventBuffer(),
		nodeInfo: &pb.NodeInfo{
			NodeId:  name,
			Address: address,
		},
		controlPlane:      controlPlane,
		predecessor:       nil,
		successor:         nil,
		heartbeatInterval: 5 * time.Second, // TODO: Configurable
		// Keep this as os.Stdout for simplicity - can be easily extended
		// to use file or other logging backends
		// Use tint for nicer output
		logger: slog.New(tint.NewHandler(
			os.Stdout,
			&tint.Options{
				Level: slog.LevelDebug,
				// GO's default reference time
				TimeFormat: "02-01-2006 15:04:05",
			},
		)),
	}

	n.connectToControlPlane()

	// Start sending heartbeats to the control plane in the background
	go n.startHeartbeat()

	return n
}

func (n *Node) connectToControlPlane() {
	neighbors, err := n.controlPlane.RegisterNode(context.Background(), n.nodeInfo)
	if err != nil {
		panic(fmt.Sprintf("Failed to register node with control plane: %w", err))
	}

	n.logger.Info("Registered node with control plane", "node_id", n.nodeInfo.NodeId, "address", n.nodeInfo.Address)

	if neighbors.Predecessor != nil {
		n.predecessor = &NodeConnection{
			Client: n.createClientConnection(neighbors.Predecessor.Address),
		}
		n.logger.Info("Set predecessor",
			"node_id", neighbors.Predecessor.NodeId,
			"address", neighbors.Predecessor.Address,
		)
	} else {
		n.logger.Info("No predecessor (this node is HEAD)")
	}

	if neighbors.Successor != nil {
		n.successor = &NodeConnection{
			Client: n.createClientConnection(neighbors.Successor.Address),
		}
		n.logger.Info("Set successor",
			"node_id", neighbors.Successor.NodeId,
			"address", neighbors.Successor.Address,
		)
	} else {
		n.logger.Info("No successor (this node is TAIL)")
	}
}

func (n *Node) IsHead() bool {
	return n.predecessor == nil
}

func (n *Node) IsTail() bool {
	return n.successor == nil
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
