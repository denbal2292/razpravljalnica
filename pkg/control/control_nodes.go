package control

import (
	"log/slog"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type registerNodeResult struct {
	predecessorId string
	successorId   string
	err           error
}

// Adds a new node to the nodes list, returns predecessor and successor IDs for the new node
func (cp *ControlPlane) registerNode(nodeInfo *pb.NodeInfo) *registerNodeResult {
	// 1. Check if a node with the same ID is already registered
	_, exists := cp.nodes[nodeInfo.NodeId]

	if exists {
		slog.Warn("RegisterNode: Node already registered",
			"node_id", nodeInfo.NodeId,
			"address", nodeInfo.Address,
		)

		return &registerNodeResult{"", "", status.Error(codes.AlreadyExists, "Node already registered")}
	}

	// 2. The node id must not be empty
	if nodeInfo.NodeId == "" {
		slog.Warn("RegisterNode: Node ID cannot be empty",
			"address", nodeInfo.Address,
		)

		return &registerNodeResult{"", "", status.Error(codes.InvalidArgument, "Node ID cannot be empty")}
	}

	// 3. Create a gRPC client to the node's NodeSyncServer
	client, err := grpc.NewClient(nodeInfo.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &registerNodeResult{"", "", err}
	}

	newNode := &NodeInfo{
		// TODO: Here we blindly trust the node's reported ID and address
		// Maybe grpc.Peer?
		Info:          nodeInfo,
		Client:        pb.NewNodeUpdateClient(client),
		LastHeartbeat: nil, // No heartbeat received yet
	}

	cp.logNodeInfo(newNode, "New node registered")

	// 4. Check if this is the first node in the chain
	if len(cp.chain) == 0 {
		// First node becomes both HEAD and TAIL (no predecessor or successor)
		cp.appendNode(newNode)

		// No pred/succ for the first node
		return &registerNodeResult{"", "", nil}
	}

	// Get the current TAIL node
	tailNode := cp.getTail()

	// Append the new node to the chain
	cp.appendNode(newNode)

	// Predecessor is the old TAIL, successor is nil
	return &registerNodeResult{tailNode.Info.NodeId, "", nil}
}

// Removes a node from the nodes list, returns successor and predecessor info of the removed node
func (cp *ControlPlane) unregisterNode(nodeInfo *pb.NodeInfo) *registerNodeResult {
	// Find the node to remove
	idxToRemove := cp.findNodeIndex(nodeInfo.NodeId)

	if idxToRemove == -1 {
		// Node not found
		slog.Error("UnregisterNode: Node not found",
			"node_id", nodeInfo.NodeId,
			"address", nodeInfo.Address,
		)
		return &registerNodeResult{"", "", status.Error(codes.NotFound, "Node not found")}
	}

	cp.logNodeInfo(cp.nodes[nodeInfo.NodeId], "Node unregistered successfully")

	// Get predecessor and successor for neighbor info
	var predId, succId string
	if idxToRemove > 0 {
		predId = cp.chain[idxToRemove-1]
	}
	if idxToRemove < len(cp.chain)-1 {
		succId = cp.chain[idxToRemove+1]
	}

	// Remove the node from the list
	cp.removeNode(idxToRemove)

	// Return predecessor and successor IDs for neighbor reconnection
	return &registerNodeResult{predId, succId, nil}
}
