package control

import (
	"context"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/hashicorp/raft"
)

// Heartbeat from a node to indicate it is alive
func (cp *ControlPlane) heartbeat(nodeInfo *pb.NodeInfo) bool {
	// Find the node and update its last heartbeat time
	node, exists := cp.nodes[nodeInfo.NodeId]

	if !exists {
		cp.logger.Warn("Heartbeat: Node not registered",
			"node_id", nodeInfo.NodeId,
			"address", nodeInfo.Address,
		)

		return false
	}

	cp.logNodeDebug(node, "Heartbeat received")

	node.LastHeartbeat = time.Now()
	return true
}

// TODO: This should apply events via Raft!
// Monitor heartbeats and remove nodes that have not sent a heartbeat in time
func (cp *ControlPlane) monitorHeartbeats() {
	ticker := time.NewTicker(cp.heartbeatInterval)
	defer ticker.Stop()

	// Periodically check for missed heartbeats by
	// reading from the ticker channel which ticks at heartbeatInterval
	for range ticker.C {
		if cp.raft.State() != raft.Leader {
			// Only the leader monitors heartbeats and removes dead nodes
			continue
		}

		cp.mu.Lock()
		now := time.Now()

		var lastAlive *NodeInfo = nil
		var deadInBetween bool = false

		activeChain := make([]string, 0, len(cp.chain))

		// 1. Create a list of active nodes in the chain
		for _, nodeId := range cp.chain {
			node, exists := cp.nodes[nodeId]
			if !exists {
				continue
			}

			// Check if the last heartbeat was within the timeout
			if now.Sub(node.LastHeartbeat) <= cp.heartbeatTimeout {
				if deadInBetween {
					// Reconnect last alive and current node
					go cp.reconnectNeighbors(lastAlive, node)
					deadInBetween = false
				}

				activeChain = append(activeChain, nodeId)
				lastAlive = node // Update last alive node for reconnection
			} else {
				// Node considered dead
				cp.logNodeInfo(node, "Node considered dead due to missed heartbeats")

				// Remove from nodes map
				delete(cp.nodes, nodeId)

				deadInBetween = true
			}
		}

		// There were dead nodes after the last alive node (TAIL died)
		if deadInBetween {
			// Reconnect last alive to nil (new TAIL)
			go cp.reconnectNeighbors(lastAlive, nil)
		}

		// 2. Update the control plane's node list
		cp.chain = activeChain
		cp.mu.Unlock()
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

		cp.logger.Info("Reconnected predecessor and successor around dead node",
			"predecessor", pred.Info.NodeId,
			"successor", succ.Info.NodeId,
		)

	} else if pred != nil {
		// TAIL node died -> predecessor becomes new TAIL
		if _, err := pred.Client.SetSuccessor(context.Background(), &pb.NodeInfoMessage{Node: nil}); err != nil {
			cp.logNodeError(pred, err, "Error updating predecessor to become TAIL")
		}

		cp.logger.Info("Updated predecessor to become new TAIL",
			"predecessor", pred.Info.NodeId,
		)

	} else if succ != nil {
		// HEAD node died -> successor becomes new HEAD
		if _, err := succ.Client.SetPredecessor(context.Background(), &pb.NodeInfoMessage{Node: nil}); err != nil {
			cp.logNodeError(succ, err, "Error updating successor to become HEAD")
		}

		cp.logger.Info("Updated successor to become new HEAD",
			"successor", succ.Info.NodeId,
		)
	} else {
		// This was the only node, nothing to do
		cp.logger.Info("All nodes are down, cluster is now empty")
	}
}
