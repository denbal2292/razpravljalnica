package server

import (
	"fmt"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

// A method which actually sets the successor connection
func (n *Node) setSuccessor(nodeInfo *pb.NodeInfo) {
	n.neighborMu.Lock()
	defer n.neighborMu.Unlock()

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
	} else {
		n.successor = successorConn
	}
}

// A method which actually sets the predecessor connection
func (n *Node) setPredecessor(nodeInfo *pb.NodeInfo) {
	n.neighborMu.Lock()
	defer n.neighborMu.Unlock()

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
}

func (n *Node) IsHead() bool {
	n.neighborMu.RLock()
	defer n.neighborMu.RUnlock()

	return n.predecessor == nil
}

func (n *Node) IsTail() bool {
	n.neighborMu.RLock()
	defer n.neighborMu.RUnlock()

	return n.successor == nil
}

func (n *Node) getSuccessorClient() pb.ChainReplicationClient {
	n.neighborMu.RLock()
	defer n.neighborMu.RUnlock()

	if n.successor == nil {
		return nil
	}

	return n.successor.client
}

func (n *Node) getPredecessorClient() pb.ChainReplicationClient {
	n.neighborMu.RLock()
	defer n.neighborMu.RUnlock()

	if n.predecessor == nil {
		return nil
	}

	return n.predecessor.client
}
