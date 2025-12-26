package server

import (
	"context"
	"time"
)

// Send heartbeat to the control plane
func (n *Node) sendHeartbeat(ctx context.Context) error {
	_, err := n.controlPlane.Heartbeat(ctx, n.nodeInfo)
	return err
}

// Heartbeat goroutine function
// TODO: Make it stoppable when the server shuts down
func (n *Node) startHeartbeat() {
	ticker := time.NewTicker(n.heartbeatInterval)
	defer ticker.Stop()

	// Periodically send heartbeats
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), n.heartbeatInterval/2)
		err := n.sendHeartbeat(ctx)
		cancel()

		if err != nil {
			n.logger.Warn("Failed to send heartbeat to control plane", "error", err)
		} else {
			n.logger.Debug("Heartbeat sent to control plane")
		}
	}
}
