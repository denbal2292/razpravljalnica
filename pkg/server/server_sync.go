package server

import (
	"context"
	"log/slog"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Send the last applied event number to the caller node (used for syncing)
func (n *Node) GetLastSequenceNumbers(ctx context.Context, empty *emptypb.Empty) (*pb.LastSequenceNumbersResponse, error) {
	lastReceived, lastApplied := n.eventBuffer.GetLastReceivedAndApplied()

	return &pb.LastSequenceNumbersResponse{
		LastSequenceNumber:         lastReceived,
		AcknowledgedSequenceNumber: lastApplied,
	}, nil
}

func (n *Node) syncWithSuccessor() {
	// 1. Get the last applied event number from the successor (it might have newer ACKs from the TAIL)
	successorClient := n.getSuccessorClient()
	lastSeqResp, err := successorClient.GetLastSequenceNumbers(context.Background(), &emptypb.Empty{})

	if err != nil {
		slog.Error("Failed to get last sequence number from successor", "error", err)
		return
	}

	slog.Info("Received last sequence numbers from successor", "last_received", lastSeqResp.LastSequenceNumber, "last_acked", lastSeqResp.AcknowledgedSequenceNumber)

	succLastAcked := lastSeqResp.AcknowledgedSequenceNumber
	succLastReceived := lastSeqResp.LastSequenceNumber

	lastAcked := n.eventBuffer.GetLastApplied()
	lastReceived := n.eventBuffer.GetLastReceived()

	slog.Info("Syncing ACKs with successor", "from", lastAcked, "to", succLastAcked)

	// 2. Apply any ACKs that the successor has but we don't
	for seq := lastAcked + 1; seq <= succLastAcked; seq++ {
		n.handleEventAcknowledgment(seq)
	}

	slog.Info("Sync with successor completed and all ACKs propagated", "upTo", succLastAcked)

	// 3. Send any missing events we have that the successor doesn't
	slog.Info("Sending missing events to successor", "from", succLastReceived+1, "to", lastReceived)

	for seq := succLastReceived + 1; seq <= lastReceived; seq++ {
		event := n.eventBuffer.GetEvent(seq)
		n.handleSyncEventReplication(event)
	}

	slog.Info("All missing events sent to successor during sync", "upTo", lastReceived)
}

// Handle transition to TAIL role (called when successor is set to nil)
func (n *Node) applyAllUnacknowledgedEvents() {

	lastReceived := n.eventBuffer.GetLastReceived()
	slog.Info("Becoming new TAIL, acknowledging all events up to last received", "upTo", lastReceived)

	// All events up to last received are now acknowledged
	for seq := n.eventBuffer.GetLastApplied() + 1; seq <= lastReceived; seq++ {
		n.handleEventAcknowledgment(seq)
	}

	slog.Info("TAIL transition complete, all events acknowledged up to last received", "upTo", lastReceived)
}
