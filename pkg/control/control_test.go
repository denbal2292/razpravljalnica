package control

import (
	"io"
	"log/slog"
	"testing"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

func TestAppendNodesToChain(t *testing.T) {
	cp := newTestControlPlane()

	nodeIds := []string{"node1", "node2", "node3"}

	// Append nodes to the chain
	for _, nodeId := range nodeIds {
		cp.appendNode(&NodeInfo{
			Info: &pb.NodeInfo{
				NodeId: nodeId,
			},
		})
	}

	// Verify chain length
	if len(cp.chain) != len(nodeIds) {
		t.Errorf("Expected chain length %d, got %d", len(nodeIds), len(cp.chain))
	}

	// Verify order of nodes in the chain
	for i, nodeId := range cp.chain {
		if nodeId != nodeIds[i] {
			t.Errorf("Expected node ID %s at position %d, got %s", nodeIds[i], i, nodeId)
		}
	}
}

func TestGetTail(t *testing.T) {
	cp := newTestControlPlane()

	// Test empty chain
	t.Run("EmptyChain", func(t *testing.T) {
		tail := cp.getTail()

		if tail != nil {
			t.Errorf("Expected nil tail for empty chain, got %v", tail)
		}
	})

	// Test with single node
	t.Run("SingleNode", func(t *testing.T) {
		node := &NodeInfo{Info: &pb.NodeInfo{NodeId: "node1"}}
		cp.appendNode(node)

		tail := cp.getTail()

		if tail == nil {
			t.Fatal("Expected non-nil tail")
		}

		if tail.Info.NodeId != "node1" {
			t.Errorf("Expected tail node1, got %s", tail.Info.NodeId)
		}
	})

	// Test with multiple nodes
	t.Run("MultipleNodes", func(t *testing.T) {
		cp := newTestControlPlane()
		nodes := []string{"node1", "node2", "node3"}

		for _, nodeId := range nodes {
			cp.appendNode(&NodeInfo{Info: &pb.NodeInfo{NodeId: nodeId}})
		}

		tail := cp.getTail()

		if tail == nil {
			t.Fatal("Expected non-nil tail")
		}

		if tail.Info.NodeId != "node3" {
			t.Errorf("Expected tail node3, got %s", tail.Info.NodeId)
		}
	})
}

func TestGetHead(t *testing.T) {
	cp := newTestControlPlane()

	// Test empty chain
	t.Run("EmptyChain", func(t *testing.T) {
		head := cp.getHead()

		if head != nil {
			t.Errorf("Expected nil head for empty chain, got %v", head)
		}
	})

	// Test with single node
	t.Run("SingleNode", func(t *testing.T) {
		node := &NodeInfo{Info: &pb.NodeInfo{NodeId: "node1"}}
		cp.appendNode(node)

		head := cp.getHead()

		if head == nil {
			t.Fatal("Expected non-nil head")
		}

		if head.Info.NodeId != "node1" {
			t.Errorf("Expected head node1, got %s", head.Info.NodeId)
		}
	})

	// Test with multiple nodes
	t.Run("MultipleNodes", func(t *testing.T) {
		cp := newTestControlPlane()
		nodes := []string{"node1", "node2", "node3"}

		for _, nodeId := range nodes {
			cp.appendNode(
				&NodeInfo{
					Info: &pb.NodeInfo{NodeId: nodeId},
				},
			)
		}

		head := cp.getHead()

		if head == nil {
			t.Fatal("Expected non-nil head")
		}

		if head.Info.NodeId != "node1" {
			t.Errorf("Expected head node1, got %s", head.Info.NodeId)
		}
	})
}

func TestFindNodeIndex(t *testing.T) {
	cp := newTestControlPlane()

	// Test empty chain
	t.Run("EmptyChain", func(t *testing.T) {
		idx := cp.findNodeIndex("node1")

		if idx != -1 {
			t.Errorf("Expected -1 for empty chain, got %d", idx)
		}
	})

	// Setup chain for remaining tests
	nodes := []string{"node1", "node2", "node3"}
	for _, nodeId := range nodes {
		cp.appendNode(&NodeInfo{Info: &pb.NodeInfo{NodeId: nodeId}})
	}

	// Test finding existing nodes
	t.Run("ExistingNodes", func(t *testing.T) {
		// Define the test cases
		tests := []struct {
			nodeId   string
			expected int
		}{
			{"node1", 0},
			{"node2", 1},
			{"node3", 2},
		}

		for _, tt := range tests {
			t.Run(tt.nodeId, func(t *testing.T) {
				idx := cp.findNodeIndex(tt.nodeId)

				if idx != tt.expected {
					t.Errorf("Expected index %d for %s, got %d", tt.expected, tt.nodeId, idx)
				}
			})
		}
	})

	// Test non-existent node
	t.Run("NonExistentNode", func(t *testing.T) {
		idx := cp.findNodeIndex("nonexistent")

		if idx != -1 {
			t.Errorf("Expected -1 for non-existent node, got %d", idx)
		}
	})
}

func TestRemoveNode(t *testing.T) {
	// Test removing head node
	nodeIds := []string{"node1", "node2", "node3"}

	appendNodes := func(cp *ControlPlane) {
		for _, nodeId := range nodeIds {
			cp.appendNode(&NodeInfo{Info: &pb.NodeInfo{NodeId: nodeId}})
		}
	}

	t.Run("RemoveHead", func(t *testing.T) {
		cp := newTestControlPlane()
		appendNodes(cp)
		cp.removeNode(0) // Remove head

		if len(cp.chain) != 2 {
			t.Errorf("Expected chain length 2, got %d", len(cp.chain))
		}

		if cp.chain[0] != "node2" {
			t.Errorf("Expected node2 as new head, got %s", cp.chain[0])
		}

		if cp.nodes["node1"] != nil {
			t.Error("node1 should be removed from nodes map")
		}

		if cp.getHead().Info.NodeId != "node2" {
			t.Errorf("Expected head node2, got %s", cp.getHead().Info.NodeId)
		}
	})

	// Test removing middle node
	t.Run("RemoveMiddle", func(t *testing.T) {
		cp := newTestControlPlane()
		appendNodes(cp)

		cp.removeNode(1) // Remove middle

		if len(cp.chain) != 2 {
			t.Errorf("Expected chain length 2, got %d", len(cp.chain))
		}

		if cp.chain[0] != "node1" || cp.chain[1] != "node3" {
			t.Errorf("Expected [node1, node3], got %v", cp.chain)
		}

		if cp.nodes["node2"] != nil {
			t.Error("node2 should be removed from nodes map")
		}
	})

	// Test removing tail node
	t.Run("RemoveTail", func(t *testing.T) {
		cp := newTestControlPlane()
		appendNodes(cp)

		cp.removeNode(2) // Remove tail

		if len(cp.chain) != 2 {
			t.Errorf("Expected chain length 2, got %d", len(cp.chain))
		}

		if cp.chain[1] != "node2" {
			t.Errorf("Expected node2 as new tail, got %s", cp.chain[1])
		}

		if cp.nodes["node3"] != nil {
			t.Error("node3 should be removed from nodes map")
		}

		if cp.getTail().Info.NodeId != "node2" {
			t.Errorf("Expected tail node2, got %s", cp.getTail().Info.NodeId)
		}
	})

	// Test removing only node
	t.Run("RemoveOnlyNode", func(t *testing.T) {
		cp := newTestControlPlane()
		cp.appendNode(&NodeInfo{Info: &pb.NodeInfo{NodeId: "node1"}})

		cp.removeNode(0)

		if len(cp.chain) != 0 {
			t.Errorf("Expected empty chain, got length %d", len(cp.chain))
		}

		if len(cp.nodes) != 0 {
			t.Errorf("Expected empty nodes map, got %d nodes", len(cp.nodes))
		}

		if cp.getHead() != nil {
			t.Error("Expected nil head after removing only node")
		}

		if cp.getTail() != nil {
			t.Error("Expected nil tail after removing only node")
		}
	})
}

func TestAppendNode_AddsToNodesMap(t *testing.T) {
	cp := newTestControlPlane()

	nodeIds := []string{"node1", "node2", "node3"}

	for _, nodeId := range nodeIds {
		cp.appendNode(&NodeInfo{
			Info: &pb.NodeInfo{NodeId: nodeId},
		})
	}

	// Verify nodes were added to the map
	for _, nodeId := range nodeIds {
		if cp.nodes[nodeId] == nil {
			t.Errorf("Node %s not found in nodes map", nodeId)
		}
	}
}

func TestHeadAndTailSameForSingleNode(t *testing.T) {
	cp := newTestControlPlane()

	node := &NodeInfo{Info: &pb.NodeInfo{NodeId: "node1"}}
	cp.appendNode(node)

	head := cp.getHead()
	tail := cp.getTail()

	if head == nil || tail == nil {
		t.Fatal("Expected non-nil head and tail")
	}

	if head.Info.NodeId != tail.Info.NodeId {
		t.Error("Expected head and tail to be the same for single node")
	}

	if head.Info.NodeId != "node1" {
		t.Errorf("Expected node1, got %s", head.Info.NodeId)
	}
}

// newTestControlPlane creates a ControlPlane instance for testing purposes
func newTestControlPlane() *ControlPlane {
	return &ControlPlane{
		nodes:             make(map[string]*NodeInfo),
		chain:             make([]string, 0),
		heartbeatInterval: 5 * time.Second,
		heartbeatTimeout:  7 * time.Second,
		logger:            slog.New(slog.NewTextHandler(io.Discard, nil)), // Discard logs in tests - to avoid seg faults
	}
}
