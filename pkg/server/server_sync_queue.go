package server

import (
	"sync"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

type ServerSyncQueue struct {
	mu        sync.RWMutex
	isSyncing bool
	syncQueue []*pb.Event   // events received during syncing
	closeChan chan struct{} // closed when sync is complete
}

func NewServerSyncQueue() *ServerSyncQueue {
	return &ServerSyncQueue{
		syncQueue: nil,
		isSyncing: false,
		mu:        sync.RWMutex{},
		closeChan: make(chan struct{}),
	}
}

func (n *Node) enterSyncState() {
	n.syncQueue.mu.Lock()
	defer n.syncQueue.mu.Unlock()

	// Mark as syncing and initialize the sync queue
	n.syncQueue.isSyncing = true
	n.syncQueue.syncQueue = make([]*pb.Event, 0)
	n.syncQueue.closeChan = make(chan struct{})
}

func (n *Node) EnqueueEvent(event *pb.Event) {
	n.syncQueue.mu.Lock()
	defer n.syncQueue.mu.Unlock()

	if !n.syncQueue.isSyncing {
		panic("enqueueEvent called when not syncing")
	}

	n.syncQueue.syncQueue = append(n.syncQueue.syncQueue, event)
}

func (n *Node) exitSyncState() {
	n.syncQueue.mu.Lock()
	defer n.syncQueue.mu.Unlock()

	// Process all queued events
	for _, event := range n.syncQueue.syncQueue {
		n.logInfoEvent(event, "Processing queued event after sync")

		// Apply the event locally and ignore errors
		n.eventBuffer.AddEvent(event)
		_ = n.applyEvent(event)

		// Acknowledge the event to predecessor
		if err := n.forwardEventToSuccessor(event); err != nil {
			n.logErrorEvent(event, err, "Failed to propagate queued event to successor after sync")
		} else {
			n.logInfoEvent(event, "Queued event propagated to successor after sync")
		}
	}

	// Mark as not syncing and clear the sync queue
	n.syncQueue.isSyncing = false
	n.syncQueue.syncQueue = nil
	close(n.syncQueue.closeChan)
}

func (n *Node) IsSyncing() bool {
	n.syncQueue.mu.RLock()
	defer n.syncQueue.mu.RUnlock()

	return n.syncQueue.isSyncing
}

func (n *Node) WaitForSyncToComplete() {
	n.syncQueue.mu.RLock()
	if !n.syncQueue.isSyncing {
		panic("WaitForSyncToComplete called when not syncing")
	}

	closeChan := n.syncQueue.closeChan
	n.syncQueue.mu.RUnlock()

	// Wait for the channel to be closed
	<-closeChan
}
