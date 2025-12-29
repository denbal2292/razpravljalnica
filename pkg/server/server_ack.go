package server

import (
	"context"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) handleEventAcknowledgment(seqNum int64) {
	go n.doSendAllUnacknowledged(seqNum)
}

func (n *Node) doSendAllUnacknowledged(seqNum int64) {
	n.ackMu.Lock()
	defer n.ackMu.Unlock()

	event := n.eventBuffer.GetEvent(seqNum)

	if seqNum == n.nextAckSeq {
		n.logger.Info("Handling next expected ACK", "sequence_number", seqNum)

		// Acknowledge the event in the buffer
		n.eventBuffer.AcknowledgeEvent(seqNum)

		// ACK is the next expected, process it immediately
		n.acknowledgeEvent(event)

		// Update the next expected ACK sequence number
		n.nextAckSeq++

		// Check if there are buffered ACKs that can now be processed
		for {
			bufferedEvent, exists := n.ackQueue[n.nextAckSeq]
			if !exists {
				break
			}

			n.logger.Info("Handling buffered ACK", "sequence_number", bufferedEvent.SequenceNumber)

			// Acknowledge the event in the buffer
			n.eventBuffer.AcknowledgeEvent(bufferedEvent.SequenceNumber)

			// Process the buffered ACK
			n.acknowledgeEvent(bufferedEvent)

			// Remove from buffer and update next expected ACK sequence number
			delete(n.ackQueue, n.nextAckSeq)
			n.nextAckSeq++
		}
	} else if seqNum > n.nextAckSeq {
		// ACK is out of order, buffer it
		n.ackQueue[event.SequenceNumber] = event
		n.logger.Info("ACK buffered due to out-of-order reception", "sequence_number", event.SequenceNumber, "expected_sequence_number", n.nextAckSeq)
	} else {
		n.logger.Warn("Received ACK for already acknowledged event - not forwarding", "sequence_number", seqNum, "last_applied", n.eventBuffer.GetLastApplied())
	}
}

func (n *Node) acknowledgeEvent(event *pb.Event) {
	if n.IsHead() {
		n.ackSync.SignalAck(event.SequenceNumber, nil)
		n.logger.Info("ACK reached HEAD successfully - sending to client", "sequence_number", event.SequenceNumber)
	} else {
		// non-HEAD: Apply it to storage (errors ignored in replication protocol)
		_ = n.applyEvent(event)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		predClient := n.getPredecessorClient()

		if predClient == nil {
			panic("ackEvent called on non-HEAD node with no predecessor?")
		}

		if _, err := predClient.AcknowledgeEvent(ctx, &pb.AcknowledgeEventRequest{
			SequenceNumber: event.SequenceNumber,
		}); err != nil {
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
