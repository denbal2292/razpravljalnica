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

	// 1. Check if a node with the same ID is already registered
	_, exists := cp.nodes[nodeInfo.NodeId]

	if exists {
		cp.logger.Warn("RegisterNode: Node already registered",
			"node_id", nodeInfo.NodeId,
			"address", nodeInfo.Address,
		)

		return nil, status.Error(codes.AlreadyExists, "Node already registered")
	}

	// 2. Create a gRPC client to the node's NodeSyncServer
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

	cp.logNodeInfo(newNode, "New node registered")

	// 3. Check if this is the first node in the chain
	if len(cp.chain) == 0 {
		// First node becomes both HEAD and TAIL (no predecessor or successor)
		cp.appendNode(newNode)

		return &pb.NeighborsInfo{
			Predecessor: nil, // No predecessor
			Successor:   nil, // No successor
		}, nil
	}

	// Get the current TAIL node
	tailNode := cp.getTail()

	// Update its successor to point to the new node
	tailNode.Client.SetSuccessor(ctx, &pb.NodeInfoMessage{Node: newNode.Info}) // Inform old TAIL about new successor

	// Append the new node to the chain
	cp.appendNode(newNode)

	return &pb.NeighborsInfo{
		Predecessor: tailNode.Info, // Old TAIL is the predecessor of the new node
		Successor:   nil,           // No successor (new TAIL)
	}, nil
}

func (cp *ControlPlane) UnregisterNode(ctx context.Context, nodeInfo *pb.NodeInfo) (*emptypb.Empty, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Find the node to remove
	idxToRemove := cp.findNodeIndex(nodeInfo.NodeId)

	if idxToRemove == -1 {
		// Node not found
		cp.logger.Error("UnregisterNode: Node not found",
			"node_id", nodeInfo.NodeId,
			"address", nodeInfo.Address,
		)
		return &emptypb.Empty{}, status.Error(codes.NotFound, "Node not found")
	}

	cp.logNodeInfo(cp.nodes[nodeInfo.NodeId], "Node unregistered successfully")

	// Get predecessor and successor nodes
	var pred, succ *NodeInfo
	if idxToRemove > 0 {
		pred = cp.nodes[cp.chain[idxToRemove-1]]
	}
	if idxToRemove < len(cp.chain)-1 {
		succ = cp.nodes[cp.chain[idxToRemove+1]]
	}

	// Remove the node from the list
	cp.removeNode(idxToRemove)

	// Update neighbors
	cp.reconnectNeighbors(pred, succ)

	return &emptypb.Empty{}, nil
}
