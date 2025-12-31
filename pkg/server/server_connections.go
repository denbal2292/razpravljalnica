package server

import (
	"context"
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

	n.cancelFunc() // Cancel any existing sending goroutine

	ctx, cancel := context.WithCancel(context.Background())
	n.cancelCtx = ctx
	n.cancelFunc = cancel

	// Open new channel for sending events
	n.sendEventChan = make(chan *pb.Event, 100)

	// Start the event replicator goroutine
	go n.eventReplicator()
}

// A method which actually sets the predecessor connection
func (n *Node) setPredecessor(nodeInfo *pb.NodeInfo) {
	n.syncMu.Lock()
	defer n.syncMu.Unlock()

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
	n.eventMu.Lock()
	n.eventQueue = make(map[int64]*pb.Event)
	n.eventMu.Unlock()
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
