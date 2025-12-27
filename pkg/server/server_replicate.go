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
		return err
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	succClient := n.getSuccessorClient()

	if succClient == nil {
		panic("forwardEventToNextNode called with no successor")
	}

	_, err := succClient.ReplicateEvent(ctx, event)
	return err
}

// (Don't call)
// gRPC method that gets called when we receive an event to replicate from the previous node
func (n *Node) ReplicateEvent(ctx context.Context, event *pb.Event) (*emptypb.Empty, error) {
	// 1. Add the event to the buffer but don't apply it yet since it's not confirmed by the tail
	n.eventBuffer.AddEvent(event)

	// Log event reception from predecessor
	n.logEventReceived(event)

	// TAIL node just applies the event and sends ACK back
	if n.IsTail() {
		// Apply event and send ACK back
		// NOTE: Storage errors are not a part of replication protocols
		n.eventBuffer.AcknowledgeEvent(event.SequenceNumber)
		_ = n.applyEvent(event)

		go func() {
			if err := n.sendAckToPredecessor(event); err != nil {
				n.logErrorEvent(event, err, "Failed to propagate ACK to predecessor")
			} else {
				// Log successful ACK propagation
				n.logInfoEvent(event, "ACK propagated to predecessor")
			}
		}()

		return &emptypb.Empty{}, nil
	}

	// 2. Forward the event to the next node in the chain asynchronously (if not TAIL)
	// TODO: These goroutines should probably be in a waitgroup to ensure proper shutdown
	go func() {
		if err := n.forwardEventToSuccessor(event); err != nil {
			// Log the error but don't return it since this is async
			n.logErrorEvent(event, err, "Failed to forward event to successor")
			// TODO: Notify control plane about failure
		}
	}()

	// 3. Return success (the ACK will be handled separately)
	n.logInfoEvent(event, "Event forwarded to successor for replication")
	return &emptypb.Empty{}, nil
}
