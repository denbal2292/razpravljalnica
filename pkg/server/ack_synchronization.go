package server

import (
	"fmt"
	"sync"
)

type AckSynchronization struct {
	mu          sync.RWMutex         // protects ackChannels
	ackChannels map[int64]chan error // event number -> channel to signal ACK receipt (nil for success, error otherwise)
}

func NewAckSynchronization() *AckSynchronization {
	return &AckSynchronization{
		ackChannels: make(map[int64]chan error),
		mu:          sync.RWMutex{},
	}
}

func (as *AckSynchronization) OpenAckChannel(seqNum int64) chan error {
	as.mu.Lock()
	defer as.mu.Unlock()

	ackChan := make(chan error, 1)
	as.ackChannels[seqNum] = ackChan
	return ackChan
}

func (as *AckSynchronization) SignalAck(seqNum int64, err error) error {
	as.mu.RLock()
	defer as.mu.RUnlock()

	ackChan, exists := as.ackChannels[seqNum]
	if exists {
		ackChan <- err
	} else {
		// TODO: Error handling
		panic(fmt.Errorf("ACK channel not found for sequence number %d", seqNum))
	}

	return nil
}

func (as *AckSynchronization) CloseAckChannel(seqNum int64) {
	as.mu.Lock()
	defer as.mu.Unlock()

	delete(as.ackChannels, seqNum)
}
