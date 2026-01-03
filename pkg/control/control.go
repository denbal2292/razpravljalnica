package control

import (
	"log/slog"
	"os"
	"sync"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/hashicorp/raft"
	"github.com/lmittmann/tint"
)

type NodeInfo struct {
	Info          *pb.NodeInfo
	Client        pb.NodeUpdateClient
	LastHeartbeat time.Time
}

type ControlPlane struct {
	pb.UnimplementedClientDiscoveryServer // For clients connecting to find HEAD and TAIL nodes
	pb.UnimplementedControlPlaneServer    // For nodes connecting to report heartbeats

	raft *raft.Raft

	mu                sync.RWMutex
	nodes             map[string]*NodeInfo // node id -> nodeinfo
	chain             []string             // nodes in order: [HEAD, ..., TAIL] (easier to get neighbors)
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	raftTimeout       time.Duration

	lastControlIndex uint64

	logger *slog.Logger
}

func NewControlPlane() *ControlPlane {
	cb := &ControlPlane{
		nodes:             make(map[string]*NodeInfo),
		chain:             make([]string, 0),
		heartbeatInterval: 5 * time.Second,
		heartbeatTimeout:  7 * time.Second,
		raftTimeout:       5 * time.Second,
		logger: slog.New(tint.NewHandler(
			os.Stdout,
			&tint.Options{
				Level: slog.LevelInfo,
				// GO's default reference time
				TimeFormat: "02-01-2006 15:04:05",
			},
		)),
	}

	// Start monitoring heartbeats
	go cb.monitorHeartbeats()

	return cb
}

func (cp *ControlPlane) getNode(nodeId string) *NodeInfo {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	return cp.nodes[nodeId]
}

// Returns two nodes for the given IDs under the same read lock to ensure consistency
func (cp *ControlPlane) getNodes(nodeId1, nodeId2 string) (*NodeInfo, *NodeInfo, bool) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var firstNode, secondNode *NodeInfo

	if nodeId1 != "" {
		first, exists := cp.nodes[nodeId1]

		if !exists {
			return nil, nil, false
		}
		firstNode = first
	}

	if nodeId2 != "" {
		second, exists := cp.nodes[nodeId2]

		if !exists {
			return nil, nil, false
		}
		secondNode = second
	}

	return firstNode, secondNode, true
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
