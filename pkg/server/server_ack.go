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
	n.logInfoEvent(event, "ACK received from successor")

	// If this is HEAD, send success to the waiting client
	if n.IsHead() {
		// TODO: The error cannot be non-nil?
		n.ackSync.SignalAck(req.SequenceNumber, nil)
		n.logInfoEvent(event, "ACK reached HEAD successfuly - sending to client")

		return &emptypb.Empty{}, nil
	}

	// 2. If not HEAD, propagate ACK to predecessor asynchronously
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	predClient := n.getPredecessorClient()

	if predClient == nil {
		panic("sendAckToPredecessor called with no predecessor")
	}

	_, err := predClient.AcknowledgeEvent(ctx, &pb.AcknowledgeEventRequest{
		SequenceNumber: event.SequenceNumber,
	})
	return err
}
