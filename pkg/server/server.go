package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Node struct {
	pb.UnimplementedMessageBoardReadsServer         // reads
	pb.UnimplementedMessageBoardWritesServer        // writes
	pb.UnimplementedMessageBoardSubscriptionsServer // subscriptions
	pb.UnimplementedChainReplicationServer          // for communication between nodes in the chain
	pb.UnimplementedNodeUpdateServer                // for control plane to notify about neighbor changes

	nodeInfo               *pb.NodeInfo            // name and address of this node
	controlPlaneAddrs      []string                // list of all control plane server addresses
	controlPlaneConnection *ControlPlaneConnection // gRPC connection to the control plane
	controlPlaneMu         sync.RWMutex            // protects controlPlaneConnection
	heartbeatInterval      time.Duration           // interval between heartbeats

	storage             *storage.Storage
	eventBuffer         *EventBuffer
	ackSync             *AckSynchronization  // for waiting for ACKs from predecessor
	subscriptionManager *SubscriptionManager // manages topic subscriptions for clients

	mu          sync.RWMutex    // protects predecessor and successor
	predecessor *NodeConnection // nil if HEAD
	successor   *NodeConnection // nil if TAIL

	ackQueue   map[int64]*pb.Event // map of sequence numbers to events for sending ACKs in order
	nextAckSeq int64               // next sequence number to ACK
	ackMu      sync.Mutex          // protects ackQueue and nextAckSeq

	eventQueue   map[int64]*pb.Event // events received out of order
	nextEventSeq int64               // next expected event sequence number
	eventMu      sync.Mutex          // protects eventQueue and nextEventSeq

	syncMu sync.Mutex // protects multiple sync operations from being run concurrently

	sendChan       chan struct{}  // sync channel for signaling new events to sen
	sendCancelChan chan struct{}  // channel to signal event replication goroutine to stop (closed when stopping)
	sendWg         sync.WaitGroup // for waiting for sync goroutine to finish

	ackGoroutineMu sync.Mutex     // protects starting/stopping ACK processor goroutine
	ackChan        chan struct{}  // sync channel for signaling new ACKs to process
	ackCancelChan  chan struct{}  // channel to signal ACK processor goroutine to stop (closed when stopping)
	ackWg          sync.WaitGroup // for waiting for ACK processor goroutine to finish

	logger *slog.Logger // logger for the node
}

type NodeConnection struct {
	address string
	client  pb.ChainReplicationClient // gRPC client to the connected node
	conn    *grpc.ClientConn          // underlying gRPC connection we can close
}

type ControlPlaneConnection struct {
	address string
	client  pb.ControlPlaneClient // gRPC client to the control plane
	conn    *grpc.ClientConn      // underlying gRPC connection we can close
}

func NewServer(name string, address string, controlPlaneAddrs []string) *Node {
	n := &Node{
		storage:     storage.NewStorage(),
		eventBuffer: NewEventBuffer(),
		nodeInfo: &pb.NodeInfo{
			NodeId:  name,
			Address: address,
		},
		controlPlaneAddrs:   controlPlaneAddrs,
		controlPlaneMu:      sync.RWMutex{},
		ackSync:             NewAckSynchronization(),
		subscriptionManager: NewSubscriptionManager(),
		ackQueue:            make(map[int64]*pb.Event),
		nextAckSeq:          0,
		ackMu:               sync.Mutex{},
		eventQueue:          make(map[int64]*pb.Event),
		nextEventSeq:        0,
		eventMu:             sync.Mutex{},
		predecessor:         nil,
		successor:           nil,

		syncMu:   sync.Mutex{},
		sendChan: make(chan struct{}),
		sendWg:   sync.WaitGroup{},

		ackGoroutineMu: sync.Mutex{},
		ackChan:        make(chan struct{}),
		ackWg:          sync.WaitGroup{},

		heartbeatInterval: 5 * time.Second,
	}

	n.connectToControlPlane()

	n.startEventReplicationGoroutine()
	n.startAckProcessorGoroutine()

	// Start sending heartbeats to the control plane in the background
	go n.startHeartbeat()

	return n
}

func (n *Node) requireHead() error {
	if !n.IsHead() {
		return status.Error(codes.FailedPrecondition, "write operation requires HEAD node")
	}

	return nil
}

func (n *Node) requireTail() error {
	if !n.IsTail() {
		return status.Error(codes.FailedPrecondition, "read operation requires TAIL node")
	}

	return nil
}

func (n *Node) startEventReplicationGoroutine() {
	// Initialize cancel channel
	n.sendCancelChan = make(chan struct{})

	// Start the event replicator goroutine
	n.sendWg.Go(n.eventReplicator)
}

func (n *Node) startAckProcessorGoroutine() {
	// Initialize cancel channel
	n.ackCancelChan = make(chan struct{})

	// Start the ACK processor goroutine
	n.ackWg.Go(n.ackProcessor)
}

func (n *Node) connectToControlPlane() {
	slog.Info("Attempting to register node with control plane")

	var neighbors *pb.NeighborsInfo
	err := n.tryControlPlaneRequest(func(client pb.ControlPlaneClient) error {
		var regErr error
		neighbors, regErr = client.RegisterNode(context.Background(), n.nodeInfo)
		return regErr
	})

	if err != nil {
		panic(fmt.Errorf("Failed to register node with control plane: %w", err))
	}

	slog.Info("Registered node with control plane", "node_id", n.nodeInfo.NodeId, "address", n.nodeInfo.Address)

	// A check only needed for stats
	if neighbors.Predecessor != nil {
		n.setPredecessor(neighbors.Predecessor)

		slog.Info("Set predecessor",
			"node_id", neighbors.Predecessor.NodeId,
			"address", neighbors.Predecessor.Address,
		)
	} else {
		slog.Info("No predecessor (this node is HEAD)")
	}

	// This should never happen (new node is always TAIL at registration)
	if neighbors.Successor != nil {
		panic("New node cannot have a successor at registration")
	} else {
		slog.Info("No successor (this node is TAIL)")
	}
}

func (n *Node) AddSubscriptionRequest(ctx context.Context, req *pb.SubscriptionNodeRequest) (*pb.AddSubscriptionResponse, error) {
	// Check if user exists before adding subscription
	if !n.storage.UserExists(req.UserId) {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return n.subscriptionManager.AddSubscriptionRequest(req)
}

type EventApplicationResult struct {
	message *pb.Message
	user    *pb.User
	topic   *pb.Topic
	err     error
}

// Apply the given event to the local storage
func (n *Node) applyEvent(event *pb.Event) *EventApplicationResult {
	var result *EventApplicationResult
	switch event.Op {
	case pb.OpType_OP_POST:
		msgRequest := event.PostMessage
		msg, err := n.storage.PostMessage(msgRequest.TopicId, msgRequest.UserId, msgRequest.Text, event.EventAt)
		n.subscriptionManager.AddEventIfNotNil(event, msg)

		result = &EventApplicationResult{message: msg, err: err}

	case pb.OpType_OP_UPDATE:
		msgRequest := event.UpdateMessage
		msg, err := n.storage.UpdateMessage(msgRequest.TopicId, msgRequest.UserId, msgRequest.MessageId, msgRequest.Text)
		n.subscriptionManager.AddEventIfNotNil(event, msg)

		result = &EventApplicationResult{message: msg, err: err}

	case pb.OpType_OP_DELETE:
		msgRequest := event.DeleteMessage
		msg, err := n.storage.DeleteMessage(msgRequest.TopicId, msgRequest.UserId, msgRequest.MessageId)
		n.subscriptionManager.AddEventIfNotNil(event, msg)

		result = &EventApplicationResult{message: msg, err: err}

	case pb.OpType_OP_LIKE:
		likeRequest := event.LikeMessage
		msg, err := n.storage.LikeMessage(likeRequest.TopicId, likeRequest.UserId, likeRequest.MessageId)
		n.subscriptionManager.AddEventIfNotNil(event, msg)

		result = &EventApplicationResult{message: msg, err: err}

	case pb.OpType_OP_CREATE_USER:
		userRequest := event.CreateUser
		user, err := n.storage.CreateUser(userRequest.Name)
		result = &EventApplicationResult{user: user, err: err}

	case pb.OpType_OP_CREATE_TOPIC:
		topicRequest := event.CreateTopic
		topic, err := n.storage.CreateTopic(topicRequest.Name)
		result = &EventApplicationResult{topic: topic, err: err}

	default:
		panic(fmt.Errorf("unknown event operation: %v", event.Op))
	}

	return result
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
	case storage.ErrInvalidLimit:
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
