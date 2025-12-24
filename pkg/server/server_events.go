package server

import (
	"context"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Replicate the event to the next node and wait for ACK from the tail (used for create/update operations)
func (n *Node) replicateAndWaitForAck(ctx context.Context, event *pb.Event) error {
	// If this is the tail, we don't send the ACK
	if n.IsTail() {
		return nil
	}

	// Register the ACK channel for this event
	ackChan := n.ackSync.OpenAckChannel(event.SequenceNumber)

	// Clean up the channel registration when done
	defer n.ackSync.CloseAckChannel(event.SequenceNumber)

	// Forward the event to the next node
	if err := n.forwardEventToNextNode(event); err != nil {
		// If forwarding fails, clean up and return the error
		return err
	}

	// Wait for the ACK with 10s timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	select {
	case err := <-ackChan:
		return err // nil if success, error otherwise
	case <-timeoutCtx.Done():
		return status.Error(codes.DeadlineExceeded, "timeout waiting for ACK from tail node")
	}
}

// Forward the event to the next node in the chain
func (n *Node) forwardEventToNextNode(event *pb.Event) error {
	_, err := n.successor.client.ReplicateEvent(context.Background(), event)
	return err
}

func (n *Node) sendAckToPrevNode(seqNum int64) error {
	// If there is no previous node, do nothing
	if n.predecessor == nil {
		return nil
	}

	ackReq := &pb.AcknowledgeEventRequest{
		SequenceNumber: seqNum,
	}
	_, err := n.predecessor.client.AcknowledgeEvent(context.Background(), ackReq)
	return err
}

// (Don't call)
// gRPC method that gets called when we receive an event to replicate from the previous node
func (n *Node) ReplicateEvent(ctx context.Context, req *pb.Event) (*emptypb.Empty, error) {
	// 1. Add the event to the buffer but don't apply it yet since it's not confirmed by the tail
	n.eventBuffer.AddEvent(req)

	if n.IsTail() {
		// If this is the tail, we can send the ACK back immediately
		_ = n.applyEvent(req)
		n.eventBuffer.AcknowledgeEvent(req.SequenceNumber)

		// Send ACK back to the previous node
		if err := n.sendAckToPrevNode(req.SequenceNumber); err != nil {
			return nil, err
		}

		return &emptypb.Empty{}, nil
	}

	// 2. Forward the event to the next node and wait for ACK
	if err := n.replicateAndWaitForAck(context.Background(), req); err != nil {
		return nil, err
	}

	// TODO: Error handling, for now we panic in the event buffer if the event doesn't match
	return &emptypb.Empty{}, nil
}

// (Don't call)
// gRPC method implementation that gets called when we receive an ACK from the next node
func (n *Node) AcknowledgeEvent(ctx context.Context, req *pb.AcknowledgeEventRequest) (*emptypb.Empty, error) {
	// 1. Acknowledge in the buffer and get the event
	// NOTE: We assume here the event exists and is the next in sequence
	event := n.eventBuffer.AcknowledgeEvent(req.SequenceNumber)

	// 2. If HEAD, send success to client immediately - we received ACK
	if n.IsHead() {
		n.ackSync.SignalAck(req.SequenceNumber, nil)
		return &emptypb.Empty{}, nil
	}

	// 3. Not HEAD, apply the event
	if err := n.applyEvent(event); err != nil {
		return nil, err
	}

	// 4. Forward the ACK to the previous node
	if err := n.sendAckToPrevNode(req.SequenceNumber); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
