package server

import (
	"context"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Replicate the event to the next node and wait for ACK from the tail (used for create/update operations)
func (n *Node) replicateAndWaitForAck(ctx context.Context, event *pb.Event) error {
	// If this is the tail, we don't propagate further
	if n.IsTail() {
		return nil
	}

	// Log the replication attempt
	n.logEventReplicated(event)

	// TODO: Put timeout in a config
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Forward the event to the next node in the chain
	_, err := n.successor.Client.ReplicateEvent(timeoutCtx, event)

	if err != nil {
		n.logFailReplicationEvent(event, err)
	} else {
		// No error means ACK received
		n.logAckReceived(event)
	}

	return err
}

// (Don't call)
// gRPC method that gets called when we receive an event to replicate from the previous node
func (n *Node) ReplicateEvent(ctx context.Context, event *pb.Event) (*emptypb.Empty, error) {
	// 1. Add the event to the buffer but don't apply it yet since it's not confirmed by the tail
	n.eventBuffer.AddEvent(event)

	// Log event reception from predecessor
	n.logEventReceived(event)

	// 2. Forward the event to the next node and wait for ACK
	if !n.IsTail() {
		// replicateAndWaitForAck will block until ACK is received from the next node
		if err := n.replicateAndWaitForAck(context.Background(), event); err != nil {
			return nil, err
		}
	}

	// 3. We received ACK from tail, now we can apply the event
	// Acknowledge in our buffer
	n.eventBuffer.AcknowledgeEvent(event.SequenceNumber)

	// Q: I don't think this check is necessary
	if n.IsHead() {
		// If HEAD, signal the waiting client and add to storage in the controller (for easier return)
		return &emptypb.Empty{}, nil
	}

	// Log event
	n.logApplyEvent(event)

	// Not HEAD, apply the event here
	if err := n.applyEvent(event); err != nil {
		return nil, err
	}

	// Log ACK propagation
	n.logAckPropagated(event)

	return &emptypb.Empty{}, nil
}
