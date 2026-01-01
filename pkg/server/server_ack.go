package server

import (
	"context"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) handleEventAcknowledgment(seqNum int64) {
	n.bufferAck(seqNum)
}

func (n *Node) bufferAck(seqNum int64) {
	n.ackMu.Lock()
	defer n.ackMu.Unlock()

	n.logger.Info("Received ACK for event", "sequence_number", seqNum)

	if seqNum < n.nextAckSeq {
		n.logger.Warn("Received ACK for already acknowledged event - not forwarding", "sequence_number", seqNum, "last_applied", n.eventBuffer.GetLastApplied())
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
	defer n.logger.Warn("Stopping ACK processor goroutine")

	n.logger.Info("Starting ACK processor goroutine")

	for {
		select {
		case <-n.ackChan:
			n.processNextAcks()

		case <-n.ackCancelCtx.Done():
			n.logger.Info("ACK processor goroutine exiting due to cancellation")
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
		n.logger.Info("Processing ACK to predecessor", "sequence_number", event.SequenceNumber)

		// Acknowledge the event in the buffer
		n.eventBuffer.AcknowledgeEvent(event.SequenceNumber)

		// Propagate the ACK to the predecessor or apply it if HEAD
		n.acknowledgeEvent(event)
		n.ackMu.Lock()
	}
}

func (n *Node) acknowledgeEvent(event *pb.Event) {
	if n.IsHead() {
		n.ackSync.SignalAck(event.SequenceNumber, nil)
		n.logger.Info("ACK reached HEAD successfully - sending to client", "sequence_number", event.SequenceNumber)
	} else {
		// non-HEAD: Apply it to storage (errors ignored in replication protocol)
		_ = n.applyEvent(event)

		predClient := n.getPredecessorClient()

		if predClient == nil {
			panic("Predecessor client is nil when trying to propagate ACK")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := predClient.AcknowledgeEvent(ctx, &pb.AcknowledgeEventRequest{
			SequenceNumber: event.SequenceNumber,
		})

		if err != nil {
			n.logger.Error("Failed to propagate ACK to predecessor", "sequence_number", event.SequenceNumber, "error", err)
		} else {
			n.logger.Info("ACK propagated to predecessor", "sequence_number", event.SequenceNumber)
		}
	}
}

func (n *Node) AcknowledgeEvent(ctx context.Context, req *pb.AcknowledgeEventRequest) (*emptypb.Empty, error) {
	n.handleEventAcknowledgment(req.SequenceNumber)
	return &emptypb.Empty{}, nil
}
