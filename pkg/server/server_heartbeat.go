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
	for {
		select {
		case <-n.heartbeatStop:
			slog.Info("Stopping heartbeat goroutine")
			return
		case <-ticker.C:
			heartbeatCtx, cancel := context.WithTimeout(context.Background(), n.heartbeatInterval/2)
			err := n.sendHeartbeat(heartbeatCtx)
			cancel()

			if err != nil {
				slog.Warn("Failed to send heartbeat to control plane", "error", err)
			} else {
				slog.Debug("Heartbeat sent to control plane")
			}
		}
	}
}

// Shutdown gracefully shuts down the node by unregistering from the control plane
func (n *Node) Shutdown() {
	slog.Warn("Shutting down node", "node_id", n.nodeInfo.NodeId)

	// Stop heartbeat goroutine
	close(n.heartbeatStop)

	// Unregister from control plane
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := n.tryControlPlaneRequest(func(client pb.ControlPlaneClient) error {
		_, err := client.UnregisterNode(ctx, n.nodeInfo)
		return err
	})

	if err != nil {
		slog.Error("Failed to unregister node from control plane", "error", err)
	} else {
		slog.Info("Successfully unregistered node from control plane")
	}

	// Close control plane connection
	n.controlPlaneMu.Lock()
	if n.controlPlaneConnection != nil {
		n.controlPlaneConnection.conn.Close()
		n.controlPlaneConnection = nil
	}
	n.controlPlaneMu.Unlock()
}
