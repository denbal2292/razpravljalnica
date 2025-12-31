package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) handleEventReplication(event *pb.Event) {
	n.bufferEvent(event)
}

func (n *Node) handleSyncEventReplication(event *pb.Event) {
	n.logger.Info("Replicating missing event to successor", "sequence_number", event.SequenceNumber)
	n.replicateEvent(event)
}

func (n *Node) bufferEvent(event *pb.Event) {
	n.eventMu.Lock()
	defer n.eventMu.Unlock()

	n.logEventReceived(event)

	if event.SequenceNumber < n.nextEventSeq {
		// This probably shouldn't happen (but better to be safe)
		n.logger.Warn("Received event for already applied sequence number - not forwarding", "sequence_number", event.SequenceNumber, "next_expected", n.nextEventSeq)
		return
	}

	// Buffer the event
	n.eventQueue[event.SequenceNumber] = event

	// Wake up the replicator goroutine to process the new event in a non-blocking way
	select {
	case n.sendChan <- struct{}{}:
		// Successfully notified the replicator
	default:
		// Replicator is already notified or busy
	}
}

// The goroutine which actually sends events to the successor
func (n *Node) eventReplicator() {
	defer n.logger.Warn("Stopping event replicator goroutine")

	n.logger.Info("Starting event replicator goroutine")

	for {
		select {
		case <-n.sendChan:
			n.replicateNextEvents()

		case <-n.cancelCtx.Done():
			n.logger.Info("Event replicator goroutine exiting due to cancellation")
			return
		}
	}
}

func (n *Node) replicateNextEvents() {
	n.eventMu.Lock()
	defer n.eventMu.Unlock()

	for {
		event, exists := n.eventQueue[n.nextEventSeq]
		if !exists {
			return
		}

		n.logger.Info("Replicating event to successor", "sequence_number", event.SequenceNumber)

		// Add the event to the buffer unless we're the HEAD (it already applied when created)
		if !n.IsHead() {
			n.eventBuffer.AddEvent(event)
		}

		n.replicateEvent(event)

		// Remove from buffer and increment expected sequence number
		delete(n.eventQueue, n.nextEventSeq)
		n.nextEventSeq++
	}
}

func (n *Node) replicateEvent(event *pb.Event) {
	if n.IsTail() {
		// If TAIL, apply the event and send ACK back
		n.handleEventAcknowledgment(event.SequenceNumber)
	} else {
		// If not TAIL, forward the event to the successor
		successorClient := n.getSuccessorClient()
		_, err := successorClient.ReplicateEvent(context.Background(), event)

		if err != nil {
			n.logger.Error("Failed to replicate event to successor", "sequence_number", event.SequenceNumber, "error", err)
		} else {
			n.logger.Info("Event replicated to successor successfully", "sequence_number", event.SequenceNumber)
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
	n.bufferEvent(event)
	return &emptypb.Empty{}, nil
}
