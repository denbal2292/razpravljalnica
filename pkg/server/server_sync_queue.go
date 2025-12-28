package server

import (
	"sync"
	"sync/atomic"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

type ServerSyncQueue struct {
	mu        sync.RWMutex
	isSyncing atomic.Bool
	syncQueue []*pb.Event   // events received during syncing
	closeChan chan struct{} // closed when sync is complete
}

func NewServerSyncQueue() *ServerSyncQueue {
	return &ServerSyncQueue{
		syncQueue: nil,
		isSyncing: atomic.Bool{},
		mu:        sync.RWMutex{},
		closeChan: make(chan struct{}),
	}
}

func (n *Node) enterSyncState() {
	n.syncMu.Lock() // Lock for syncing (to block event writes)

	n.syncQueue.mu.Lock()
	defer n.syncQueue.mu.Unlock()

	// Mark as syncing and initialize the sync queue
	n.syncQueue.isSyncing.Store(true)
	n.syncQueue.syncQueue = make([]*pb.Event, 0)
	n.syncQueue.closeChan = make(chan struct{})
}

func (n *Node) EnqueueEvent(event *pb.Event) {
	n.syncQueue.mu.Lock()
	defer n.syncQueue.mu.Unlock()

	if !n.IsSyncing() {
		panic("enqueueEvent called when not syncing")
	}

	n.syncQueue.syncQueue = append(n.syncQueue.syncQueue, event)
}

func (n *Node) exitSyncState() {
	n.syncMu.Unlock() // Unlock syncing (to allow event writes)
	n.syncQueue.mu.Lock()
	defer n.syncQueue.mu.Unlock()

	closeChan := n.syncQueue.closeChan

	// Mark as not syncing
	n.syncQueue.isSyncing.Store(false)

	// Process all queued events
	for _, event := range n.syncQueue.syncQueue {
		n.logInfoEvent(event, "Processing queued event after sync")
		n.handleEventReplication(event)
	}

	// Clear the sync queue
	n.syncQueue.syncQueue = make([]*pb.Event, 0)

	n.logger.Info("All queued events processed after sync", "count", len(n.syncQueue.syncQueue))
	close(closeChan)
}

func (n *Node) IsSyncing() bool {
	return n.syncQueue.isSyncing.Load()
}

func (n *Node) WaitForSyncToComplete() {
	n.syncQueue.mu.RLock()
	if !n.IsSyncing() {
		panic("WaitForSyncToComplete called when not syncing")
	}

	closeChan := n.syncQueue.closeChan
	n.syncQueue.mu.RUnlock()

	// Wait for the channel to be closed
	<-closeChan
}
