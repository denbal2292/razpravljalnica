package server

import (
	"fmt"
	"sync"
)

type AckSynchronization struct {
	mu          sync.RWMutex
	ackChannels map[int64]chan error // event number -> channel to signal ACK receipt (nil for success, error otherwise)
}

func NewAckSynchronization() *AckSynchronization {
	return &AckSynchronization{
		ackChannels: make(map[int64]chan error),
		mu:          sync.RWMutex{},
	}
}

func (n *AckSynchronization) OpenAckChannel(seqNum int64) chan error {
	n.mu.Lock()
	defer n.mu.Unlock()

	ackChan := make(chan error, 1)
	n.ackChannels[seqNum] = ackChan
	return ackChan
}

func (n *AckSynchronization) SignalAck(seqNum int64, err error) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	ackChan, exists := n.ackChannels[seqNum]
	if exists {
		ackChan <- err
	} else {
		// TODO: Error handling
		panic(fmt.Sprintf("ACK channel not found for sequence number %d", seqNum))
	}

	return nil
}

func (n *AckSynchronization) CloseAckChannel(seqNum int64) {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.ackChannels, seqNum)
}
