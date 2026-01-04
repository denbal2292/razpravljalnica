package control

import (
	"log/slog"
	"sync"
	"time"

	"github.com/denbal2292/razpravljalnica/pkg/control/gui"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/hashicorp/raft"
)

type NodeInfo struct {
	Info          *pb.NodeInfo
	Client        pb.NodeUpdateClient
	LastHeartbeat *time.Time
}

type ControlPlane struct {
	pb.UnimplementedClientDiscoveryServer // For clients connecting to find HEAD and TAIL nodes
	pb.UnimplementedControlPlaneServer    // For nodes connecting to report heartbeats

	raft *raft.Raft // Raft instance (set after creation)

	mu                sync.RWMutex
	nodes             map[string]*NodeInfo // node id -> nodeinfo
	chain             []string             // nodes in order: [HEAD, ..., TAIL] (easier to get neighbors)
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	raftTimeout       time.Duration

	lastControlIndex uint64

	logger *slog.Logger

	// For GUI stats
	nodeID   string
	grpcAddr string
	raftAddr string
}

func NewControlPlane() *ControlPlane {
	cb := &ControlPlane{
		nodes:             make(map[string]*NodeInfo),
		chain:             make([]string, 0),
		heartbeatInterval: 5 * time.Second,
		heartbeatTimeout:  7 * time.Second,
		raftTimeout:       5 * time.Second,
	}

	// Start monitoring heartbeats
	go cb.monitorHeartbeats()

	return cb
}

func (cp *ControlPlane) SetNodeInfo(nodeID, grpcAddr, raftAddr string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.nodeID = nodeID
	cp.grpcAddr = grpcAddr
	cp.raftAddr = raftAddr
}

// GetStats returns a snapshot of control plane statistics for the GUI.
func (cp *ControlPlane) GetStats() gui.ControlStatsSnapshot {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var raftState, raftLeader string
	if cp.raft != nil {
		raftState = cp.raft.State().String()
		raftLeader = string(cp.raft.Leader())
	} else {
		raftState = "Initializing"
		raftLeader = ""
	}

	// Build chain node info
	chainNodes := make([]gui.ChainNodeInfo, 0, len(cp.chain))
	for i, nodeID := range cp.chain {
		node := cp.nodes[nodeID]
		if node == nil {
			continue
		}

		// Determine role
		role := "MIDDLE"
		if len(cp.chain) == 1 {
			role = "SINGLE"
		} else if i == 0 {
			role = "HEAD"
		} else if i == len(cp.chain)-1 {
			role = "TAIL"
		}

		chainNodes = append(chainNodes, gui.ChainNodeInfo{
			NodeID:  nodeID,
			Address: node.Info.Address,
			Role:    role,
		})
	}

	return gui.ControlStatsSnapshot{
		NodeID:        cp.nodeID,
		GRPCAddr:      cp.grpcAddr,
		RaftAddr:      cp.raftAddr,
		RaftState:     raftState,
		RaftLeader:    raftLeader,
		ChainNodes:    chainNodes,
		RegisteredCPs: 3, // Static cluster size
		TotalNodes:    len(cp.chain),
	}
}

func (cp *ControlPlane) getNode(nodeId string) *NodeInfo {
	return cp.nodes[nodeId]
}

func (cp *ControlPlane) getTail() *NodeInfo {
	if len(cp.chain) == 0 {
		return nil
	}
	tailId := cp.chain[len(cp.chain)-1]
	return cp.nodes[tailId]
}

func (cp *ControlPlane) getHead() *NodeInfo {
	if len(cp.chain) == 0 {
		return nil
	}
	headId := cp.chain[0]
	return cp.nodes[headId]
}

func (cp *ControlPlane) appendNode(newNode *NodeInfo) {
	cp.chain = append(cp.chain, newNode.Info.NodeId)
	cp.nodes[newNode.Info.NodeId] = newNode
}

func (cp *ControlPlane) findNodeIndex(nodeId string) int {
	for idx, id := range cp.chain {
		if id == nodeId {
			return idx
		}
	}

	return -1
}

func (cp *ControlPlane) removeNode(nodeIdx int) {
	nodeId := cp.chain[nodeIdx]
	cp.chain = append(cp.chain[:nodeIdx], cp.chain[nodeIdx+1:]...)
	delete(cp.nodes, nodeId)
}

func (cp *ControlPlane) SetRaft(r *raft.Raft) {
	cp.raft = r
}
