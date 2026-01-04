package control

import (
	"context"
	"log/slog"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Heartbeat from a node to indicate it is alive
func (cp *ControlPlane) Heartbeat(context context.Context, nodeInfo *pb.NodeInfo) (*emptypb.Empty, error) {
	if err := cp.ensureLeader(); err != nil {
		return nil, err
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Find the node and update its last heartbeat time
	node, exists := cp.nodes[nodeInfo.NodeId]

	if !exists {
		slog.Warn("Heartbeat: Node not registered",
			"node_id", nodeInfo.NodeId,
			"address", nodeInfo.Address,
		)

		return nil, status.Error(codes.NotFound, "node not registered")
	}

	cp.logNodeDebug(node, "Heartbeat received")

	now := time.Now()
	node.LastHeartbeat = &now

	return &emptypb.Empty{}, nil
}

func (cp *ControlPlane) updateChain(idsToRemove []string) struct{} {
	// Create a set for faster lookup
	toRemoveSet := make(map[string]struct{})
	for _, id := range idsToRemove {
		toRemoveSet[id] = struct{}{}
	}

	// Build new chain without removed nodes
	newChain := make([]string, 0, len(cp.chain))
	for _, nodeId := range cp.chain {
		if _, shouldRemove := toRemoveSet[nodeId]; shouldRemove {
			// Remove from nodes map
			delete(cp.nodes, nodeId)
		} else {
			// Keep in chain
			newChain = append(newChain, nodeId)
		}
	}

	// Update the chain
	cp.chain = newChain

	return struct{}{}
}

// Monitor heartbeats and remove nodes that have not sent a heartbeat in time
func (cp *ControlPlane) monitorHeartbeats() {
	ticker := time.NewTicker(cp.heartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		// Only the leader monitors heartbeats
		if cp.raft.State() != raft.Leader {
			continue
		}

		cp.mu.Lock()

		// Identify dead nodes and capture state before removal
		deadNodeIds, oldChain := cp.identifyDeadNodes()

		cp.mu.Unlock()

		if len(deadNodeIds) == 0 {
			continue
		}

		// Apply removals via Raft
		if err := cp.removeNodesViaRaft(deadNodeIds); err != nil {
			slog.Error("Error applying heartbeat removals via Raft", "error", err)
		}

		// Reconnect chain around removed nodes
		cp.reconnectChain(oldChain, deadNodeIds)
	}
}

// identifyDeadNodes finds nodes that haven't sent heartbeats within the timeout
// Returns the list of dead node IDs and a snapshot of the current chain
func (cp *ControlPlane) identifyDeadNodes() ([]string, []string) {
	now := time.Now()
	var deadNodes []string

	for _, nodeId := range cp.chain {
		node := cp.nodes[nodeId]

		if node.LastHeartbeat == nil {
			// No heartbeat received yet
			node.LastHeartbeat = &now // Set to now to avoid immediate removal
			continue
		}

		if now.Sub(*node.LastHeartbeat) > cp.heartbeatTimeout {
			cp.logNodeInfo(node, "Node considered dead due to missed heartbeats")
			deadNodes = append(deadNodes, nodeId)
		}
	}

	// Snapshot current chain state
	oldChain := make([]string, len(cp.chain))
	copy(oldChain, cp.chain)

	return deadNodes, oldChain
}

// removeNodesViaRaft applies a Raft command to remove the specified nodes
func (cp *ControlPlane) removeNodesViaRaft(nodeIds []string) error {
	raftCommand := &pb.RaftCommand{
		Op:          pb.RaftCommandType_OP_UPDATE_CHAIN,
		IdsToRemove: nodeIds,
		CreatedAt:   timestamppb.New(time.Now()),
	}

	_, err := applyRaftCommand[struct{}](cp, raftCommand)
	return err
}

// reconnectChain updates neighbor connections for nodes adjacent to removed nodes
func (cp *ControlPlane) reconnectChain(oldChain []string, removedIds []string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Build set for fast lookup
	removedSet := make(map[string]struct{})
	for _, id := range removedIds {
		removedSet[id] = struct{}{}
	}

	var lastAlive *NodeInfo
	var gapDetected bool

	// Walk through old chain to find reconnection points
	for _, nodeId := range oldChain {
		node, exists := cp.nodes[nodeId]
		_, removed := removedSet[nodeId]

		if removed || !exists {
			// This node was removed - mark gap
			gapDetected = true
			continue
		}

		// This node is alive
		if gapDetected && lastAlive != nil {
			// Reconnect across the gap
			go cp.reconnectNeighbors(lastAlive, node)
			gapDetected = false
		}

		lastAlive = node
	}

	// Handle trailing removed nodes (TAIL died)
	if gapDetected && lastAlive != nil {
		go cp.reconnectNeighbors(lastAlive, nil)
	}

	// Handle leading removed nodes (HEAD died)
	if len(oldChain) > 0 && len(cp.chain) > 0 {
		_, headDied := removedSet[oldChain[0]]

		if headDied {
			newHead := cp.getHead()
			go cp.reconnectNeighbors(nil, newHead)
		}
	}

	// Handle all nodes removed
	if len(cp.chain) == 0 {
		go cp.reconnectNeighbors(nil, nil)
	}
}

// Reconnect the chain around the dead node
// TODO: Maybe add timeouts for these RPCs?
func (cp *ControlPlane) reconnectNeighbors(pred *NodeInfo, succ *NodeInfo) {
	// The node will be removed from the list in monitorHeartbeats,
	// here we just need to update neighbors (reconnect the chain)
	if pred != nil && succ != nil {
		// Middle node died
		// NOTE: Connect predecessor before successor to make sure when syncing starts, the successor knows about its new predecessor
		// Successor is now connected to predecessor of the dead node
		if _, err := succ.Client.SetPredecessor(context.Background(), &pb.NodeInfoMessage{Node: pred.Info}); err != nil {
			cp.logNodeError(succ, err, "Error updating successor")
		}

		// Predecessor is now connected to successor of the dead node
		if _, err := pred.Client.SetSuccessor(context.Background(), &pb.NodeInfoMessage{Node: succ.Info}); err != nil {
			cp.logNodeError(pred, err, "Error updating predecessor")
		}

		slog.Info("Reconnected predecessor and successor around dead node",
			"predecessor", pred.Info.NodeId,
			"successor", succ.Info.NodeId,
		)

	} else if pred != nil {
		// TAIL node died -> predecessor becomes new TAIL
		if _, err := pred.Client.SetSuccessor(context.Background(), &pb.NodeInfoMessage{Node: nil}); err != nil {
			cp.logNodeError(pred, err, "Error updating predecessor to become TAIL")
		}

		slog.Info("Updated predecessor to become new TAIL",
			"predecessor", pred.Info.NodeId,
		)

	} else if succ != nil {
		// HEAD node died -> successor becomes new HEAD
		if _, err := succ.Client.SetPredecessor(context.Background(), &pb.NodeInfoMessage{Node: nil}); err != nil {
			cp.logNodeError(succ, err, "Error updating successor to become HEAD")
		}

		slog.Info("Updated successor to become new HEAD",
			"successor", succ.Info.NodeId,
		)
	} else {
		// This was the only node, nothing to do
		slog.Info("All nodes are down, cluster is now empty")
	}
}
