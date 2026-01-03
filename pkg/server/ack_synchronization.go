package server

import (
	"fmt"
	"sync"
)

type AckSynchronization struct {
	mu          sync.RWMutex                           // protects ackChannels
	ackChannels map[int64]chan *EventApplicationResult // event number -> channel to signal ACK receipt (nil for success, error otherwise)
}

func NewAckSynchronization() *AckSynchronization {
	return &AckSynchronization{
		ackChannels: make(map[int64]chan *EventApplicationResult),
		mu:          sync.RWMutex{},
	}
}

func (as *AckSynchronization) OpenAckChannel(seqNum int64) chan *EventApplicationResult {
	as.mu.Lock()
	defer as.mu.Unlock()

	ackChan := make(chan *EventApplicationResult, 1)
	as.ackChannels[seqNum] = ackChan
	return ackChan
}

func (as *AckSynchronization) SignalAckIfWaiting(seqNum int64, result *EventApplicationResult) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	ackChan, exists := as.ackChannels[seqNum]
	if exists {
		ackChan <- result
	}
}

func (as *AckSynchronization) CloseAckChannel(seqNum int64) {
	as.mu.Lock()
	defer as.mu.Unlock()

	delete(as.ackChannels, seqNum)
}

func (as *AckSynchronization) WaitForAck(seqNum int64) *EventApplicationResult {
	as.mu.RLock()
	ackChan, exists := as.ackChannels[seqNum]
	as.mu.RUnlock()

	if !exists {
		panic(fmt.Errorf("ACK channel not found for sequence number %d", seqNum))
	}

	result := <-ackChan
	return result
}
