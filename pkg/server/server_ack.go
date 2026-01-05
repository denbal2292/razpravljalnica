package server

import (
	"context"
	"log/slog"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) handleEventAcknowledgment(seqNum int64) {
	n.bufferAck(seqNum)
}

func (n *Node) bufferAck(seqNum int64) {
	n.ackMu.Lock()
	defer n.ackMu.Unlock()

	slog.Info("Received ACK for event", "sequence_number", seqNum)

	if seqNum < n.nextAckSeq {
		slog.Warn("Received ACK for already acknowledged event - not forwarding", "sequence_number", seqNum, "last_applied", n.eventBuffer.GetLastApplied())
		return
	}

	// Retrieve the event from the buffer
	event := n.eventBuffer.GetEvent(seqNum)

	// Buffer it
	n.ackQueue[event.SequenceNumber] = event

	// Wake up the ACK processor goroutine to process the new ACK in a non-blocking way
	select {
	case n.ackChan <- struct{}{}:
		// Successfully notified the ACK processor
	default:
		// ACK processor is already notified or busy
	}
}

func (n *Node) ackProcessor() {
	defer slog.Warn("Stopping ACK processor goroutine")

	slog.Info("Starting ACK processor goroutine")

	// Check if there are ACKs to process right away
	n.processNextAcks()

	for {
		select {
		case <-n.ackChan:
			n.processNextAcks()

		case <-n.ackCancelCtx.Done():
			slog.Info("ACK processor goroutine exiting due to cancellation")
			return
		}
	}
}

func (n *Node) processNextAcks() {
	n.ackMu.Lock()
	defer n.ackMu.Unlock()

	for {
		event, exists := n.ackQueue[n.nextAckSeq]
		if !exists {
			return
		}

		// Remove from buffer and increment expected ACK sequence number
		delete(n.ackQueue, n.nextAckSeq)
		n.nextAckSeq++
		n.ackMu.Unlock()

		// Since we have only one ACK processor goroutine, it's safe to process the ACK without further locking
		slog.Info("Processing ACK to predecessor", "sequence_number", event.SequenceNumber)

		// Acknowledge the event in the buffer
		n.eventBuffer.AcknowledgeEvent(event.SequenceNumber)

		// Propagate the ACK to the predecessor or apply it if HEAD
		n.acknowledgeEvent(event)
		n.ackMu.Lock()
	}
}

func (n *Node) acknowledgeEvent(event *pb.Event) {
	// Apply the event locally and get the result
	result := n.applyEvent(event)

	if n.IsHead() {
		n.ackSync.SignalAckIfWaiting(event.SequenceNumber, result)
		slog.Info("ACK reached HEAD successfully - sending to client", "sequence_number", event.SequenceNumber)
	} else {
		stream := n.getPredecessorAckStream()

		if stream == nil {
			slog.Error("No predecessor ACK stream available, cannot propagate ACK", "sequence_number", event.SequenceNumber)
			return
		}

		err := stream.Send(&pb.AcknowledgeEventRequest{
			SequenceNumber: event.SequenceNumber,
		})

		if err != nil {
			slog.Error("Failed to propagate ACK to predecessor", "sequence_number", event.SequenceNumber, "error", err)
		} else {
			slog.Info("ACK propagated to predecessor", "sequence_number", event.SequenceNumber)
		}
	}
}

func (n *Node) AcknowledgeEvent(ctx context.Context, req *pb.AcknowledgeEventRequest) (*emptypb.Empty, error) {
	n.handleEventAcknowledgment(req.SequenceNumber)
	return &emptypb.Empty{}, nil
}

// gRPC streaming method for receiving acknowledgments from the successor
func (n *Node) AcknowledgeEventStream(stream pb.ChainReplication_AcknowledgeEventStreamServer) error {
	slog.Info("AcknowledgeEventStream started - receiving ACKs from successor")

	for {
		req, err := stream.Recv()
		if err != nil {
			slog.Info("AcknowledgeEventStream closed", "error", err)
			return err
		}

		n.handleEventAcknowledgment(req.SequenceNumber)
	}
}
