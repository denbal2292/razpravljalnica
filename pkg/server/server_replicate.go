package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) handleEventReplication(event *pb.Event) {
	go n.doForwardAllEvents(event)
}

func (n *Node) handleSyncEventReplication(event *pb.Event) {
	n.logger.Info("Replicating missing event to successor", "sequence_number", event.SequenceNumber)
	n.sendEventChan <- event
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
		n.sendEventChan <- event

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
			n.sendEventChan <- bufferedEvent

			// Remove from buffer and update next expected sequence number
			delete(n.eventQueue, n.nextEventSeq)
			n.nextEventSeq++
		}
	} else if event.SequenceNumber > n.nextEventSeq {
		// Event is out of order, buffer it
		n.eventQueue[event.SequenceNumber] = event
		n.logger.Info("Event buffered due to out-of-order reception", "sequence_number", event.SequenceNumber, "expected_sequence_number", n.nextEventSeq)
	} else {
		// This probably shouldn't happen (but better to be safe)
		n.logger.Warn("Received event for already applied sequence number - not forwarding", "sequence_number", event.SequenceNumber, "next_expected", n.nextEventSeq)
	}
}

func (n *Node) eventReplicator() {
	defer n.logger.Warn("Exiting event replicator!")
	defer close(n.sendEventChan)

	n.logger.Info("Starting event replicator goroutine")

	for {
		select {
		case event := <-n.sendEventChan:
			if n.IsTail() {
				// If this is TAIL, apply the event and send ACK back
				n.handleEventAcknowledgment(event.SequenceNumber)
			} else {
				// else, forward the event to the successor
				successorClient := n.getSuccessorClient()
				_, err := successorClient.ReplicateEvent(context.Background(), event)

				if err != nil {
					n.logger.Error("Failed to replicate event to successor", "sequence_number", event.SequenceNumber, "error", err)
				} else {
					n.logger.Info("Event replicated to successor successfully", "sequence_number", event.SequenceNumber)
				}
			}

		case <-n.cancelCtx.Done():
			n.logger.Info("Event replicator goroutine exiting due to cancellation")
			return
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
