package server

import (
	"log/slog"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

func (n *Node) logInfoEvent(event *pb.Event, message string) {
	slog.Info(
		message,
		"operation", event.Op.String(),
		"sequence_number", event.SequenceNumber,
	)
}

func (n *Node) logErrorEvent(event *pb.Event, err error, message string) {
	slog.Error(
		message,
		"error", err,
		"operation", event.Op.String(),
		"sequence_number", event.SequenceNumber,
	)
}

// infos
func (n *Node) logApplyEvent(event *pb.Event) {
	n.logInfoEvent(event, "Commiting event")
}

func (n *Node) logEventReceived(event *pb.Event) {
	n.logInfoEvent(event, "Received new event")
}
