package server

import pb "github.com/denbal2292/razpravljalnica/pkg/pb"

func (n *Node) logInfoEvent(event *pb.Event, message string) {
	n.logger.Info(
		message,
		"operation", event.Op.String(),
		"sequence_number", event.SequenceNumber,
	)
}

func (n *Node) logErrorEvent(event *pb.Event, err error, message string) {
	n.logger.Error(
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

func (n *Node) logAckReceived(event *pb.Event) {
	n.logInfoEvent(event, "ACK received from successor")
}

func (n *Node) logAckPropagated(event *pb.Event) {
	n.logInfoEvent(event, "ACK propagated to predecessor")
}

func (n *Node) logEventReplicated(event *pb.Event) {
	n.logInfoEvent(event, "Replicating event to successor")
}

// errors
func (n *Node) logFailReplicationEvent(event *pb.Event, err error) {
	n.logErrorEvent(event, err, "Failed to replicate to successor")
}
