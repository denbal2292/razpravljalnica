package server

import (
	"context"
	"log/slog"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) handleEventReplication(event *pb.Event) {
	n.bufferEvent(event)
}

func (n *Node) handleSyncEventReplication(event *pb.Event) {
	slog.Info("Replicating missing event to successor", "sequence_number", event.SequenceNumber)
	n.replicateEvent(event)
}

func (n *Node) bufferEvent(event *pb.Event) {
	n.eventMu.Lock()
	defer n.eventMu.Unlock()

	n.logEventReceived(event)

	if event.SequenceNumber < n.nextEventSeq {
		// This probably shouldn't happen (but better to be safe)
		slog.Warn("Received event for already applied sequence number - not forwarding", "sequence_number", event.SequenceNumber, "next_expected", n.nextEventSeq)
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
	defer slog.Info("Stopping event replicator goroutine")

	slog.Info("Starting event replicator goroutine")

	// Check if there are events to replicate right away
	n.replicateNextEvents()

	for {
		select {
		case <-n.sendChan:
			n.replicateNextEvents()

		case <-n.sendCancelChan:
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

		slog.Info("Replicating event to successor", "sequence_number", event.SequenceNumber)

		// Remove from buffer and increment expected sequence number
		delete(n.eventQueue, n.nextEventSeq)
		n.nextEventSeq++

		// Add the event to the buffer unless we're the HEAD (it already applied when created)
		n.eventBuffer.AddEvent(event)

		// Since there is only one goroutine sending events, we can unlock here
		// NOTE: The go routine will be blocked for the duration of event replication
		// so no out-of-order sending can happen (no other goroutine can send events)
		n.eventMu.Unlock()

		n.replicateEvent(event)
		n.eventMu.Lock()
	}
}

func (n *Node) replicateEvent(event *pb.Event) {
	if n.IsTail() {
		// If TAIL, apply the event and send ACK back
		n.handleEventAcknowledgment(event.SequenceNumber)
	} else {
		// If not TAIL, forward the event to the successor via stream
		stream := n.getSuccessorReplicateStream()

		if stream == nil {
			slog.Error("No successor replicate stream available, cannot replicate event", "sequence_number", event.SequenceNumber)
			return
		}

		err := stream.Send(event)

		if err != nil {
			slog.Error("Failed to replicate event to successor", "sequence_number", event.SequenceNumber, "error", err)
		} else {
			slog.Info("Event replicated to successor successfully", "sequence_number", event.SequenceNumber)
		}
	}
}

func (n *Node) handleEventReplicationAndWaitForAck(event *pb.Event) *EventApplicationResult {
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

// gRPC streaming method for receiving events from the predecessor
func (n *Node) ReplicateEventStream(stream pb.ChainReplication_ReplicateEventStreamServer) error {
	slog.Info("ReplicateEventStream started - receiving events from predecessor")

	for {
		event, err := stream.Recv()
		if err != nil {
			slog.Info("ReplicateEventStream closed", "error", err)
			return err
		}

		n.bufferEvent(event)
	}
}
