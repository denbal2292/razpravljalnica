package control

import (
	"context"
	"log"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Heartbeat from a node to indicate it is alive
func (cp *ControlPlane) Heartbeat(context context.Context, nodeInfo *pb.NodeInfo) (*emptypb.Empty, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Find the node and update its last heartbeat time
	// Here we don't use a map for nodes since we need to keep them in order
	for _, node := range cp.nodes {
		if node.Info.NodeId == nodeInfo.NodeId && node.Info.Address == nodeInfo.Address {
			node.LastHeartbeat = time.Now()
			return &emptypb.Empty{}, nil
		}
	}

	return nil, status.Error(codes.NotFound, "node not registered")
}

// Monitor heartbeats and remove nodes that have not sent a heartbeat in time
func (cp *ControlPlane) monitorHeartbeats() {
	ticker := time.NewTicker(cp.heartbeatInterval)
	defer ticker.Stop()

	// Periodically check for missed heartbeats by
	// reading from the ticker channel which ticks at heartbeatInterval
	for range ticker.C {
		cp.mu.Lock()
		now := time.Now()
		activeNodes := make([]*NodeInfo, 0, len(cp.nodes))

		// Create a list of active nodes
		for idx, node := range cp.nodes {
			// Check if the last heartbeat was within the timeout
			if now.Sub(node.LastHeartbeat) <= cp.heartbeatTimeout {
				activeNodes = append(activeNodes, node)
			} else {
				// Node considered dead, log or handle as needed
				// TODO: Some other logger?
				log.Printf("Node %s (%s) considered dead due to missed heartbeats", node.Info.NodeId, node.Info.Address)

				// Get predecessor and successor
				var pred, succ *NodeInfo
				if idx > 0 {
					pred = cp.nodes[idx-1]
				}
				if idx < len(cp.nodes)-1 {
					succ = cp.nodes[idx+1]
				}

				// Handle dead node (update neighbors, etc.)
				go cp.reconnectNeighbors(pred, succ)
			}
		}

		cp.nodes = activeNodes
		cp.mu.Unlock()
	}
}

// Reconnect the chain around the dead
// TODO: We return nil for predecessor/successor to mean no change,
// but elsewhere we use nil to mean no predecessor/successor at all
func (cp *ControlPlane) reconnectNeighbors(pred *NodeInfo, succ *NodeInfo) {
	// The node will be removed from the list in monitorHeartbeats,
	// here we just need to update neighbors (reconnect the chain)

	if pred != nil && succ != nil {
		// Middle node died
		// Predecessor is now connected to successor
		_, err := pred.Client.UpdateNeighbors(context.Background(),
			&pb.NeighborsInfo{
				Predecessor: nil,       // Predecessor remains the same
				Successor:   succ.Info, // New successor is the dead node's successor
			},
		)
		if err != nil {
			log.Printf("Error updating predecessor %s: %v", pred.Info.NodeId, err)
		}

		// Successor is now connected to predecessor
		_, err = succ.Client.UpdateNeighbors(context.Background(),
			&pb.NeighborsInfo{
				Predecessor: pred.Info, // New predecessor is the dead node's predecessor
				Successor:   nil,       // Successor remains the same
			},
		)
		if err != nil {
			log.Printf("Error updating successor %s: %v", succ.Info.NodeId, err)
		}
	} else if pred != nil {
		// TAIL node died -> predecessor becomes new TAIL
		_, err := pred.Client.UpdateNeighbors(context.Background(),
			&pb.NeighborsInfo{
				Predecessor: nil, // Predecessor remains the same
				Successor:   nil, // Now no successor
			},
		)

		if err != nil {
			log.Printf("Error updating predecessor %s to become TAIL: %v", pred.Info.NodeId, err)
		}
	} else if succ != nil {
		// HEAD node died -> successor becomes new HEAD
		_, err := succ.Client.UpdateNeighbors(context.Background(),
			&pb.NeighborsInfo{
				Predecessor: nil, // Now no predecessor
				Successor:   nil, // Successor remains the same
			},
		)

		if err != nil {
			log.Printf("Error updating successor %s to become HEAD: %v", succ.Info.NodeId, err)
		}
	} else {
		// This was the only node, nothing to do
		log.Printf("All nodes are down, cluster is now empty")
	}
}
