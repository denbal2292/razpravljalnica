package server

import (
	"fmt"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

// A method which actually sets the successor connection
func (n *Node) setSuccessor(nodeInfo *pb.NodeInfo) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 1. Try to close existing connection if any
	if n.successor != nil {
		n.successor.conn.Close()
	}

	// If nil, just set to nil and return
	if nodeInfo == nil {
		n.successor = nil
		return
	}

	// 2. Create new connection
	successorConn, err := n.createClientConnection(nodeInfo.Address)

	if err != nil {
		panic(fmt.Errorf("Failed to create client connection to successor: %w", err))
	}

	n.successor = successorConn
}

// A method which actually sets the predecessor connection
func (n *Node) setPredecessor(nodeInfo *pb.NodeInfo) {
	n.eventMu.Lock()
	defer n.eventMu.Unlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	// 1. Try to close existing connection if any
	if n.predecessor != nil {
		n.predecessor.conn.Close()
	}

	// If nil, just set to nil and return
	if nodeInfo == nil {
		n.predecessor = nil
		return
	}

	// 2. Create new connection
	predecessorConn, err := n.createClientConnection(nodeInfo.Address)

	if err != nil {
		panic(fmt.Errorf("Failed to create client connection to predecessor: %w", err))
	} else {
		n.predecessor = predecessorConn
	}

	// 3. Clear any pending events (we are resent events)
	n.eventQueue = make(map[int64]*pb.Event)
}

func (n *Node) IsHead() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.predecessor == nil
}

func (n *Node) IsTail() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.successor == nil
}

func (n *Node) getSuccessorClient() pb.ChainReplicationClient {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.successor == nil {
		return nil
	}

	return n.successor.client
}

func (n *Node) getPredecessorClient() pb.ChainReplicationClient {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.predecessor == nil {
		return nil
	}

	return n.predecessor.client
}

func (n *Node) getSuccessorReplicateStream() pb.ChainReplication_ReplicateEventStreamClient {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.successor == nil {
		return nil
	}

	return n.successor.replicateStream
}

func (n *Node) getPredecessorAckStream() pb.ChainReplication_AcknowledgeEventStreamClient {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.predecessor == nil {
		return nil
	}

	return n.predecessor.ackStream
}
