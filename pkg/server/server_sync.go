package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Stream events from the event buffer starting from the requested sequence number
func (n *Node) SyncEvents(syncEvents *pb.SyncEventsRequest, stream grpc.ServerStreamingServer[pb.Event]) error {
	startIndex := syncEvents.FromSequenceNumber
	n.logger.Info("Starting event sync", "from_sequence_number", startIndex)

	// TODO: This is very unsafe (we don't even have the lock here)
	// - the node needs to stop accepting new events while syncing
	// For now, we assume now new events are being added during sync
	for _, event := range n.eventBuffer.events[startIndex:] {
		if err := stream.Send(event); err != nil {
			n.logger.Error("Failed to send event during sync", "error", err)
			return err
		}

		n.logger.Debug("Sent event during sync", "sequence_number", event.SequenceNumber)
	}

	return nil
}

// Send the last applied event number to the caller node
func (n *Node) GetLastSequenceNumber(context context.Context, empty *emptypb.Empty) (*pb.LastSequenceNumberResponse, error) {
	return &pb.LastSequenceNumberResponse{
		SequenceNumber: n.eventBuffer.GetLastApplied(),
	}, nil
}
