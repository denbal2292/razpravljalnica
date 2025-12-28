package server

import (
	"context"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Replicate the event to the next node and wait for ACK from the tail (used for create/update operations)
func (n *Node) replicateAndWaitForAck(event *pb.Event) error {
	// If this is the tail, we don't propagate further
	if n.IsTail() {
		return nil
	}

	// Register the ACK channel for this event (will be closed when ACK is received)
	ackChan := n.ackSync.OpenAckChannel(event.SequenceNumber)

	// Clean up the channel after we're done
	defer n.ackSync.CloseAckChannel(event.SequenceNumber)

	// Log the replication attempt
	n.logInfoEvent(event, "Replicating event to successor")

	// Forward the event to the next node in the chain
	if err := n.forwardEventToSuccessor(event); err != nil {
		n.logErrorEvent(event, err, "Failed to replicate to successor")
		// TODO: Notify control plane about failure
	}

	// No error means the event was received by the next node
	// Now wait for the ACK from the tail (or error)
	if err := <-ackChan; err != nil {
		n.logErrorEvent(event, err, "Failed to receive ACK from tail")
		return err
	}

	// Log successful ACK receipt
	n.logInfoEvent(event, "ACK received from successor")
	return nil
}

func (n *Node) forwardEventToSuccessor(event *pb.Event) error {
	n.eventMu.Lock()
	defer n.eventMu.Unlock()

	if event.SequenceNumber == n.nextEventSeq {
		n.logger.Info("Sending next expected event to successor", "sequence_number", event.SequenceNumber)
		// Event is the next expected, send it immediately
		if err := n.sendEventToSuccessor(event); err != nil {
			n.logErrorEvent(event, err, "Failed to send next event to successor")
		}

		// Update the next expected sequence number
		n.nextEventSeq++

		// Check if there are buffered events that can now be sent
		for {
			bufferedEvent, exists := n.eventQueue[n.nextEventSeq]
			if !exists {
				break
			}

			// Send the buffered event
			if err := n.sendEventToSuccessor(bufferedEvent); err != nil {
				n.logErrorEvent(bufferedEvent, err, "Failed to send buffered event to successor")
			}

			// Remove from buffer and update next expected sequence number
			delete(n.eventQueue, n.nextEventSeq)
			n.nextEventSeq++
		}
	} else if event.SequenceNumber > n.nextEventSeq {
		// Event is out of order, buffer it
		n.eventQueue[event.SequenceNumber] = event
		// Log buffering
		n.logger.Info("Event buffered due to out-of-order reception", "sequence_number", event.SequenceNumber, "expected_sequence_number", n.nextEventSeq)
	} else {
		// Duplicate or old event, used for syncing with a new successor
		n.logger.Info("Sending duplicate/old event to successor", "sequence_number", event.SequenceNumber)
		if err := n.sendEventToSuccessor(event); err != nil {
			n.logErrorEvent(event, err, "Failed to send duplicate/old event to successor")
		}
	}

	return nil
}

func (n *Node) sendEventToSuccessor(event *pb.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	succClient := n.getSuccessorClient()

	if succClient == nil {
		panic("sendEventToSuccessor called with no successor")
	}

	_, err := succClient.ReplicateEvent(ctx, event)
	return err
}

// (Don't call)
// gRPC method that gets called when we receive an event to replicate from the previous node
func (n *Node) ReplicateEvent(ctx context.Context, event *pb.Event) (*emptypb.Empty, error) {
	if n.IsSyncing() {
		// If syncing, enqueue the event to be processed later
		n.EnqueueEvent(event)
		n.logger.Info("Node is syncing, event enqueued", "sequence_number", event.SequenceNumber)
		return &emptypb.Empty{}, nil
	}

	// Add the event to the buffer
	n.eventBuffer.AddEvent(event)
	n.logEventReceived(event)

	if n.IsTail() {
		// Apply event and send ACK back if this is the tail
		n.eventBuffer.AcknowledgeEvent(event.SequenceNumber)
		_ = n.applyEvent(event)
		go func() {
			if err := n.sendAckToPredecessor(event); err != nil {
				n.logErrorEvent(event, err, "Failed to propagate ACK to predecessor")
			} else {
				n.logInfoEvent(event, "TAIL: ACK propagated to predecessor")
			}
		}()
		return &emptypb.Empty{}, nil
	}

	// Forward the event to the next node asynchronously
	go func() {
		if err := n.forwardEventToSuccessor(event); err != nil {
			n.logErrorEvent(event, err, "Failed to forward event to successor")
		}
	}()

	n.logInfoEvent(event, "Event forwarded to successor")
	return &emptypb.Empty{}, nil
}
