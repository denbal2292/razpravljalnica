package server

import (
	"github.com/denbal2292/razpravljalnica/pkg/server/gui"
)

// GetStats returns a snapshot of all server statistics.
func (n *Node) GetStats() gui.ServerStatsSnapshot {
	n.mu.RLock()
	pred := n.predecessor
	succ := n.successor
	n.mu.RUnlock()

	cpAddr := "N/A"
	n.controlPlaneMu.RLock()
	if n.controlPlaneConnection != nil {
		cpAddr = n.controlPlaneConnection.address
	}
	n.controlPlaneMu.RUnlock()

	processed, applied := n.eventBuffer.GetLastReceivedAndApplied()

	// Determine role based on neighbors
	var role string
	hasPred := pred != nil
	hasSucc := succ != nil

	switch {
	case !hasPred && !hasSucc:
		role = "SINGLE"
	case !hasPred && hasSucc:
		role = "HEAD"
	case hasPred && !hasSucc:
		role = "TAIL"
	case hasPred && hasSucc:
		role = "MIDDLE"
	}

	var predAddr, succAddr string
	if pred != nil {
		predAddr = pred.address
	}
	if succ != nil {
		succAddr = succ.address
	}

	// Get storage stats
	messagesCount := n.storage.GetMessageCount()
	topicsCount := n.storage.GetTopicCount()
	usersCount := n.storage.GetUserCount()

	return gui.ServerStatsSnapshot{
		NodeID:           n.nodeInfo.NodeId,
		NodeAddr:         n.nodeInfo.Address,
		Role:             role,
		ControlPlaneAddr: cpAddr,
		PredecessorAddr:  predAddr,
		SuccessorAddr:    succAddr,
		EventsProcessed:  processed,
		EventsApplied:    applied,
		MessagesStored:   messagesCount,
		TopicsCount:      topicsCount,
		UsersCount:       usersCount,
	}
}

// Legacy methods - kept for backward compatibility if needed elsewhere
func (n *Node) GetProcessedAndAppliedEventsCount() (int64, int64) {
	return n.eventBuffer.GetLastReceivedAndApplied()
}

func (n *Node) GetPredecessorAndSuccessorAddresses() (string, string) {
	stats := n.GetStats()
	return stats.PredecessorAddr, stats.SuccessorAddr
}
