package control

import (
	"context"
	"sync"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NodeInfo struct {
	Info          *pb.NodeInfo
	Client        pb.NodeSyncClient
	LastHeartbeat time.Time
}

type ControlPlane struct {
	pb.ClientDiscoveryServer           // For clients connecting to find HEAD and TAIL nodes
	pb.UnimplementedControlPlaneServer // For nodes connecting to report heartbeats

	mu                sync.RWMutex
	nodes             []*NodeInfo // nodes in order: [HEAD, ..., TAIL] (easier to get neighbors)
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
}

func NewControlPlane() *ControlPlane {
	cb := &ControlPlane{
		nodes:             make([]*NodeInfo, 0),
		heartbeatInterval: 5 * time.Second,
		heartbeatTimeout:  15 * time.Second,
	}

	// Start monitoring heartbeats
	go cb.monitorHeartbeats()

	return cb
}

func (cp *ControlPlane) RegisterNode(ctx context.Context, nodeInfo *pb.NodeInfo) (*pb.NeighborsInfo, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Create gRPC client to the node's NodeSyncServer
	client, err := grpc.NewClient(nodeInfo.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	newNode := &NodeInfo{
		// TODO: Here we blindly trust the node's reported ID and address
		// Maybe grpc.Peer?
		Info:          nodeInfo,
		Client:        pb.NewNodeSyncClient(client),
		LastHeartbeat: time.Now(), // Initialize heartbeat time so it isn't considered dead immediately
	}

	// Check if this is the first node to register
	if len(cp.nodes) == 0 {
		// First node becomes both HEAD and TAIL (no predecessor or successor)
		cp.nodes = append(cp.nodes, newNode)

		return &pb.NeighborsInfo{
			Predecessor: nil, // No predecessor
			Successor:   nil, // No successor
		}, nil
	}

	// Get the current TAIL node
	tailNode := cp.nodes[len(cp.nodes)-1]

	// Update its successor to point to the new node
	tailNode.Client.UpdateNeighbors(ctx,
		&pb.NeighborsInfo{
			Predecessor: nil,      // Predecessor remains the same
			Successor:   nodeInfo, // New node is the successor
		},
	)

	// New node's predecessor is the old TAIL
	cp.nodes = append(cp.nodes, newNode)

	return &pb.NeighborsInfo{
		Predecessor: tailNode.Info, // Old TAIL is the predecessor of the new node
		Successor:   nil,           // No successor (new TAIL)
	}, nil
}
