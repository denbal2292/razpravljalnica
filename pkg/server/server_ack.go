package server

import (
	"context"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) AcknowledgeEvent(ctx context.Context, req *pb.AcknowledgeEventRequest) (*emptypb.Empty, error) {
	// 1. Acknowledge the event in the buffer and retrieve it
	event := n.eventBuffer.AcknowledgeEvent(req.SequenceNumber)

	if event == nil {
		n.logger.Warn("AcknowledgeEvent called for already acknowledged event", "sequence_number", req.SequenceNumber)
		return &emptypb.Empty{}, nil
	}

	n.logInfoEvent(event, "ACK received from successor")

	// If this is HEAD, send success to the waiting client
	if n.IsHead() {
		// TODO: The error cannot be non-nil?
		// TODO: This could be an ACK from propagating to a parent and not actually for the client
		n.ackSync.SignalAck(req.SequenceNumber, nil)
		// n.logInfoEvent(event, "ACK reached HEAD successfuly - sending to client")

		return &emptypb.Empty{}, nil
	}

	// 2. If not HEAD, propagate ACK to predecessor asynchronously
	// Apply it to storage
	_ = n.applyEvent(event)

	go func() {
		if err := n.sendAckToPredecessor(event); err != nil {
			n.logErrorEvent(event, err, "Failed to propagate ACK to predecessor")

			// TODO: Notify control plane about failure
		}
	}()

	// 3. Return success
	return &emptypb.Empty{}, nil
}

func (n *Node) sendAckToPredecessor(event *pb.Event) error {
	n.ackMu.Lock()
	defer n.ackMu.Unlock()

	// Check if the event sequence number matches the expected next ACK
	if event.SequenceNumber == n.nextAckSeq {
		// Send ACK immediately
		if err := n.sendAck(event); err != nil {
			return err
		}
		n.nextAckSeq++

		// Check for any buffered ACKs that can now be sent
		for {
			bufferedEvent, exists := n.ackQueue[n.nextAckSeq]

			// Stop if there's no buffered ACK for the next expected sequence number
			if !exists {
				break
			}

			// Send the buffered ACK and remove it from the queue
			if err := n.sendAck(bufferedEvent); err != nil {
				return err
			}
			delete(n.ackQueue, n.nextAckSeq)

			// Move to the next expected ACK sequence number
			n.nextAckSeq++
		}
	} else {
		// Buffer the ACK for later sending (some ACKs are out of order)
		n.ackQueue[event.SequenceNumber] = event
	}

	return nil
}

func (n *Node) sendAck(event *pb.Event) error {
	if n.IsHead() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	predClient := n.getPredecessorClient()

	if predClient == nil {
		panic("sendAck called with no predecessor")
	}

	_, err := predClient.AcknowledgeEvent(ctx, &pb.AcknowledgeEventRequest{
		SequenceNumber: event.SequenceNumber,
	})
	return err
}
