package server

import (
	"context"
	"fmt"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) handleEventReplication(event *pb.Event) {
	go n.doForwardAllEvents(event)
}

func (n *Node) handleSyncEventReplication(event *pb.Event) {
	n.logger.Info("Replicating missing event to successor", "sequence_number", event.SequenceNumber)
	n.forwardEvent(event)
}

func (n *Node) doForwardAllEvents(event *pb.Event) {
	n.syncMu.RLock()
	defer n.syncMu.RUnlock()

	n.eventMu.Lock()
	defer n.eventMu.Unlock()

	n.logEventReceived(event)

	if event.SequenceNumber == n.nextEventSeq {
		n.logger.Info("Handling next expected event", "sequence_number", event.SequenceNumber)

		// Add the event to the buffer unless we're the HEAD (it already applied when created)
		if !n.IsHead() {
			n.eventBuffer.AddEvent(event)
		}

		// Event is the next expected, send it immediately
		n.forwardEvent(event)

		// Update the next expected sequence number
		n.nextEventSeq++

		// Check if there are buffered events that can now be sent
		for {
			bufferedEvent, exists := n.eventQueue[n.nextEventSeq]
			if !exists {
				break
			}

			n.logger.Info("Handling buffered event", "sequence_number", bufferedEvent.SequenceNumber)

			// Add the event to the buffer unless we're the HEAD (it already applied when created)
			if !n.IsHead() {
				n.eventBuffer.AddEvent(bufferedEvent)
			}

			// Send the buffered event
			n.forwardEvent(bufferedEvent)

			// Remove from buffer and update next expected sequence number
			delete(n.eventQueue, n.nextEventSeq)
			n.nextEventSeq++
		}
	} else if event.SequenceNumber > n.nextEventSeq {
		// Event is out of order, buffer it
		n.eventQueue[event.SequenceNumber] = event
		n.logger.Info("Event buffered due to out-of-order reception", "sequence_number", event.SequenceNumber, "expected_sequence_number", n.nextEventSeq)
	} else {
		panic(fmt.Errorf("Trying to forward an already applied event %d, expected %d", event.SequenceNumber, n.nextEventSeq))
	}
}

func (n *Node) forwardEvent(event *pb.Event) {
	if n.IsTail() {
		// If this is the tail, apply the event and send ACK back
		n.handleEventAcknowledgment(event.SequenceNumber)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		succClient := n.getSuccessorClient()

		if succClient == nil {
			panic("sendEventToSuccessor called on non-tail node with no successor?")
		}

		if _, err := succClient.ReplicateEvent(ctx, event); err != nil {
			n.logger.Error("Failed to forward event to successor", "error", err)
		} else {
			n.logger.Info("Event forwarded to successor", "sequence_number", event.SequenceNumber)
		}
	}
}

func (n *Node) handleEventReplicationAndWaitForAck(event *pb.Event) error {
	// Register the ACK channel for this event (will be closed when ACK is received)
	n.ackSync.OpenAckChannel(event.SequenceNumber)

	// Clean up the channel after we're done
	defer n.ackSync.CloseAckChannel(event.SequenceNumber)

	n.handleEventReplication(event)
	return n.ackSync.WaitForAck(event.SequenceNumber)
}

// (Don't call)
// gRPC method that gets called when we receive an event to replicate from the previous node
func (n *Node) ReplicateEvent(ctx context.Context, event *pb.Event) (*emptypb.Empty, error) {
	n.handleEventReplication(event)
	return &emptypb.Empty{}, nil
}
