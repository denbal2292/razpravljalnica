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

// Check if there is a pending client waiting for ACK on the given sequence number
func (as *AckSynchronization) HasAckChannel(seqNum int64) bool {
	as.mu.RLock()
	defer as.mu.RUnlock()

	_, exists := as.ackChannels[seqNum]
	return exists
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
		// TODO: This could happen during node syncing!!!
		fmt.Printf("ACK channel not found for sequence number %d", seqNum)
		// panic(fmt.Errorf("ACK channel not found for sequence number %d", seqNum))
	}

	return nil
}

func (as *AckSynchronization) CloseAckChannel(seqNum int64) {
	as.mu.Lock()
	defer as.mu.Unlock()

	delete(as.ackChannels, seqNum)
}

func (as *AckSynchronization) WaitForAck(seqNum int64) error {
	as.mu.RLock()
	ackChan, exists := as.ackChannels[seqNum]
	as.mu.RUnlock()

	if !exists {
		panic(fmt.Errorf("ACK channel not found for sequence number %d", seqNum))
	}

	err := <-ackChan
	return err
}
