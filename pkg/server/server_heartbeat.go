package server

import (
	"context"
	"log/slog"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

// Send heartbeat to the control plane
func (n *Node) sendHeartbeat(ctx context.Context) error {
	return n.tryControlPlaneRequest(func(client pb.ControlPlaneClient) error {
		_, err := client.Heartbeat(ctx, n.nodeInfo)
		return err
	})
}

// Heartbeat goroutine function
func (n *Node) startHeartbeat() {
	ticker := time.NewTicker(n.heartbeatInterval)
	defer ticker.Stop()

	// Periodically send heartbeats
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), n.heartbeatInterval/2)
		err := n.sendHeartbeat(ctx)
		cancel()

		if err != nil {
			slog.Warn("Failed to send heartbeat to control plane", "error", err)
		} else {
			slog.Debug("Heartbeat sent to control plane")
		}
	}
}
