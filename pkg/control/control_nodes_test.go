package control

import (
	"context"
	"testing"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegisterNode(t *testing.T) {
	cp := newTestControlPlane()

	nodeInfo := &pb.NodeInfo{
		NodeId:  "node1",
		Address: "localhost:5001",
	}

	t.Run("SuccessfulNodeRegistration", func(t *testing.T) {
		// First registration should succeed
		registeredNodeNeighbors, err := cp.RegisterNode(context.Background(), nodeInfo)

		if err != nil {
			t.Fatalf("Expected no error when registering first node, got %v", err)
		}

		if len(cp.chain) != 1 {
			t.Errorf("Expected chain length 1 after first registration, got %d", len(cp.chain))
		}

		if cp.chain[0] != "node1" {
			t.Errorf("Expected chain to contain node1, got %v", cp.chain)
		}

		if registeredNodeNeighbors.Predecessor != nil {
			t.Errorf("Expected no predecessor for first node, got %v", registeredNodeNeighbors.Predecessor)
		}

		if registeredNodeNeighbors.Successor != nil {
			t.Errorf("Expected no successor for first node, got %v", registeredNodeNeighbors.Successor)
		}
	})

	// Second registration with same ID should fail
	t.Run("DuplicateNodeRegistration", func(t *testing.T) {
		_, err := cp.RegisterNode(context.Background(), nodeInfo)

		if status.Code(err) != codes.AlreadyExists {
			t.Fatal("Expected error when registering duplicate node, got nil")
		}

		if len(cp.chain) != 1 {
			t.Errorf("Expected chain length 1 after duplicate registration, got %d", len(cp.chain))
		}

		if cp.chain[0] != "node1" {
			t.Errorf("Expected chain to still contain node1, got %v", cp.chain)
		}
	})

	// Other test cases would require mocking the gRPC client creation
}

func TestRegisterNode_ChainStateAfterRegistration(t *testing.T) {
	// Note: We can't fully test RegisterNode without mocking the gRPC client
	// but we can test the chain state management logic
	// Test if first node becomes both HEAD and TAIL
	t.Run("FirstNodeBecomesHeadAndTail", func(t *testing.T) {
		cp := newTestControlPlane()

		// Manually simulate what RegisterNode does for the first node
		nodeInfo := &pb.NodeInfo{
			NodeId:  "node1",
			Address: "localhost:5001",
		}

		newNode := &NodeInfo{
			Info: nodeInfo,
		}

		// Check chain is empty before
		if len(cp.chain) != 0 {
			t.Errorf("Expected empty chain initially, got %d", len(cp.chain))
		}

		cp.RegisterNode(context.Background(), newNode.Info)

		// Verify chain has one node
		if len(cp.chain) != 1 {
			t.Errorf("Expected chain length 1, got %d", len(cp.chain))
		}

		// Verify it's both head and tail
		head := cp.getHead()
		tail := cp.getTail()

		if head == nil || tail == nil {
			t.Fatal("Expected non-nil head and tail")
		}

		if head.Info.NodeId != "node1" {
			t.Errorf("Expected head node1, got %s", head.Info.NodeId)
		}

		if tail.Info.NodeId != "node1" {
			t.Errorf("Expected tail node1, got %s", tail.Info.NodeId)
		}
	})

	// Test if multiple nodes form correct chain and HEAD/TAIL are correct
	t.Run("MultipleNodesFormChain", func(t *testing.T) {
		cp := newTestControlPlane()

		// Simulate registering multiple nodes
		nodeIds := []string{"node1", "node2", "node3"}

		for _, nodeId := range nodeIds {
			cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: nodeId})
		}

		// Verify chain length
		if len(cp.chain) != 3 {
			t.Errorf("Expected chain length 3, got %d", len(cp.chain))
		}

		// Verify head is first node
		if cp.getHead().Info.NodeId != "node1" {
			t.Errorf("Expected head node1, got %s", cp.getHead().Info.NodeId)
		}

		// Verify tail is last node
		if cp.getTail().Info.NodeId != "node3" {
			t.Errorf("Expected tail node3, got %s", cp.getTail().Info.NodeId)
		}

		// Verify all nodes are in the map
		for _, nodeId := range nodeIds {
			if cp.nodes[nodeId] == nil {
				t.Errorf("Node %s not found in nodes map", nodeId)
			}
		}
	})
}

func TestUnregisterNode_NonExistentNode(t *testing.T) {
	cp := newTestControlPlane()

	// Add some nodes
	cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node1"})
	cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node2"})

	// Try to unregister a node that doesn't exist
	_, err := cp.UnregisterNode(context.Background(), &pb.NodeInfo{NodeId: "nonexistent"})

	if err == nil {
		t.Fatal("Expected error for non-existent node")
	}

	statusErr, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got %v", err)
	}

	if statusErr.Code() != codes.NotFound {
		t.Errorf("Expected NotFound error code, got %v", statusErr.Code())
	}

	// Verify chain length didn't change
	if len(cp.chain) != 2 {
		t.Errorf("Expected chain length 2, got %d", len(cp.chain))
	}
}

func TestUnregisterNode_ChainStateAfterUnregistration(t *testing.T) {
	// Test unregistering the only node in the chain
	t.Run("UnregisterOnlyNode", func(t *testing.T) {
		cp := newTestControlPlane()

		// Register single node
		cp.RegisterNode(context.Background(), &pb.NodeInfo{
			NodeId: "node1",
		})

		// Unregister it
		_, err := cp.UnregisterNode(context.Background(), &pb.NodeInfo{NodeId: "node1"})

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify chain is empty
		if len(cp.chain) != 0 {
			t.Errorf("Expected empty chain, got %d nodes", len(cp.chain))
		}

		if len(cp.nodes) != 0 {
			t.Errorf("Expected empty nodes map, got %d nodes", len(cp.nodes))
		}

		if cp.getHead() != nil {
			t.Error("Expected nil head after unregistering only node")
		}

		if cp.getTail() != nil {
			t.Error("Expected nil tail after unregistering only node")
		}
	})

	// Test unregistering head node in a multi-node chain
	t.Run("UnregisterHeadNode", func(t *testing.T) {
		cp := newTestControlPlane()

		// Register three nodes
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node1"})
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node2"})
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node3"})

		// Unregister head
		_, err := cp.UnregisterNode(context.Background(), &pb.NodeInfo{NodeId: "node1"})

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify chain length
		if len(cp.chain) != 2 {
			t.Errorf("Expected chain length 2, got %d", len(cp.chain))
		}

		// Verify new head
		if cp.getHead().Info.NodeId != "node2" {
			t.Errorf("Expected new head node2, got %s", cp.getHead().Info.NodeId)
		}

		// Verify tail unchanged
		if cp.getTail().Info.NodeId != "node3" {
			t.Errorf("Expected tail node3, got %s", cp.getTail().Info.NodeId)
		}

		// Verify node1 removed from map
		if _, exists := cp.nodes["node1"]; exists {
			t.Error("node1 should be removed from nodes map")
		}
	})

	// Test unregistering tail node in a multi-node chain
	t.Run("UnregisterTailNode", func(t *testing.T) {
		cp := newTestControlPlane()

		// Register three nodes
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node1"})
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node2"})
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node3"})

		// Unregister tail
		_, err := cp.UnregisterNode(context.Background(), &pb.NodeInfo{NodeId: "node3"})

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify chain length
		if len(cp.chain) != 2 {
			t.Errorf("Expected chain length 2, got %d", len(cp.chain))
		}

		// Verify head unchanged
		if cp.getHead().Info.NodeId != "node1" {
			t.Errorf("Expected head node1, got %s", cp.getHead().Info.NodeId)
		}

		// Verify new tail
		if cp.getTail().Info.NodeId != "node2" {
			t.Errorf("Expected new tail node2, got %s", cp.getTail().Info.NodeId)
		}

		// Verify node3 removed from map
		if cp.nodes["node3"] != nil {
			t.Error("node3 should be removed from nodes map")
		}
	})

	// Test unregistering middle node in a multi-node chain
	t.Run("UnregisterMiddleNode", func(t *testing.T) {
		cp := newTestControlPlane()

		// Register three nodes
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node1"})
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node2"})
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node3"})

		// Unregister middle node
		_, err := cp.UnregisterNode(context.Background(), &pb.NodeInfo{NodeId: "node2"})

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify chain length
		if len(cp.chain) != 2 {
			t.Errorf("Expected chain length 2, got %d", len(cp.chain))
		}

		// Verify chain order
		if cp.chain[0] != "node1" || cp.chain[1] != "node3" {
			t.Errorf("Expected chain [node1, node3], got %v", cp.chain)
		}

		// Verify head unchanged
		if cp.getHead().Info.NodeId != "node1" {
			t.Errorf("Expected head node1, got %s", cp.getHead().Info.NodeId)
		}

		// Verify tail unchanged
		if cp.getTail().Info.NodeId != "node3" {
			t.Errorf("Expected tail node3, got %s", cp.getTail().Info.NodeId)
		}

		// Verify node2 removed from map
		if cp.nodes["node2"] != nil {
			t.Error("node2 should be removed from nodes map")
		}
	})

	// Test unregistering multiple nodes sequentially
	t.Run("UnregisterNodesSequentially", func(t *testing.T) {
		cp := newTestControlPlane()

		// Add three nodes
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node1"})
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node2"})
		cp.RegisterNode(context.Background(), &pb.NodeInfo{NodeId: "node3"})

		// Unregister them one by one
		cp.UnregisterNode(context.Background(), &pb.NodeInfo{NodeId: "node1"})

		if len(cp.chain) != 2 {
			t.Errorf("After removing node1, expected chain length 2, got %d", len(cp.chain))
		}

		cp.UnregisterNode(context.Background(), &pb.NodeInfo{NodeId: "node2"})

		if len(cp.chain) != 1 {
			t.Errorf("After removing node2, expected chain length 1, got %d", len(cp.chain))
		}

		cp.UnregisterNode(context.Background(), &pb.NodeInfo{NodeId: "node3"})

		if len(cp.chain) != 0 {
			t.Errorf("After removing node3, expected chain length 0, got %d", len(cp.chain))
		}

		if len(cp.nodes) != 0 {
			t.Errorf("Expected empty nodes map, got %d nodes", len(cp.nodes))
		}
	})
}
