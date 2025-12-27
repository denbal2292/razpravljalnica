package control

import (
	"context"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

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
		Client:        pb.NewNodeUpdateClient(client),
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
	tailNode.Client.SetSuccessor(ctx, &pb.NodeInfoMessage{Node: newNode.Info}) // Inform old TAIL about new successor

	// New node's predecessor is the old TAIL
	cp.nodes = append(cp.nodes, newNode)

	return &pb.NeighborsInfo{
		Predecessor: tailNode.Info, // Old TAIL is the predecessor of the new node
		Successor:   nil,           // No successor (new TAIL)
	}, nil
}

func (cp *ControlPlane) UnregisterNode(ctx context.Context, nodeInfo *pb.NodeInfo) (*emptypb.Empty, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Find the node to remove
	var idxToRemove int = -1
	for idx, node := range cp.nodes {
		if node.Info.NodeId == nodeInfo.NodeId && node.Info.Address == nodeInfo.Address {
			idxToRemove = idx
			break
		}
	}

	if idxToRemove == -1 {
		// Node not found
		return &emptypb.Empty{}, status.Error(codes.NotFound, "Node not found")
	}

	// Get predecessor and successor nodes
	var pred, succ *NodeInfo
	if idxToRemove > 0 {
		pred = cp.nodes[idxToRemove-1]
	}
	if idxToRemove < len(cp.nodes)-1 {
		succ = cp.nodes[idxToRemove+1]
	}

	// Update neighbors
	cp.reconnectNeighbors(pred, succ)

	// Remove the node from the list
	cp.nodes = append(cp.nodes[:idxToRemove], cp.nodes[idxToRemove+1:]...)

	return &emptypb.Empty{}, nil
}
