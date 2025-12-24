package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) ReplicateEvent(ctx context.Context, req *pb.Event) (*emptypb.Empty, error) {
	// 1. Add the event to the buffer but don't apply it yet since it's not confirmed by the tail
	n.eventBuffer.AddEvent(req)

	// 2. Forward the event to the next node if exists
	// TODO: Handle error and probably do it in a goroutine
	_ = n.forwardEventToNextNode(req)

	// TODO: Error handling, for now we panic in the event buffer
	return &emptypb.Empty{}, nil
}

func (n *Node) AcknowledgeEvent(ctx context.Context, req *pb.AcknowledgeEventRequest) (*emptypb.Empty, error) {
	// 1. Acknowledge in the buffer and get the event
	event := n.eventBuffer.AcknowledgeEvent(req.SequenceNumber)

	// 2. Apply the event to the storage
	n.applyEvent(event)

	// 3. Send ack to the previous node if exists
	// TODO: Handle error and probably do it in a goroutine
	_ = n.sendAckToPrevNode(req.SequenceNumber)

	// TODO: Error handling, for now we panic in the event buffer
	return &emptypb.Empty{}, nil
}

func (n *Node) forwardEventToNextNode(event *pb.Event) error {
	// If there is no next node, do nothing
	if n.successor == nil {
		return nil
	}

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
