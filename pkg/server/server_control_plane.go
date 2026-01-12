package server

import (
	"fmt"
	"log/slog"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// connectToControlPlaneServer establishes a gRPC connection to a specific control plane server
func (n *Node) connectToControlPlaneServer(addr string) (pb.ControlPlaneClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		slog.Debug("Failed to create gRPC client", "address", addr, "error", err)
		return nil, nil, err
	}

	client := pb.NewControlPlaneClient(conn)
	return client, conn, nil
}

// Try current server first, then all others if it fails due to connection
// issues or the server not being the leader
func (n *Node) tryControlPlaneRequest(requestFunc func(pb.ControlPlaneClient) error) error {
	n.controlPlaneMu.RLock()
	currentConnection := n.controlPlaneConnection
	n.controlPlaneMu.RUnlock()

	// Try current client first if we have one
	if currentConnection != nil {
		err := requestFunc(currentConnection.client)

		if err == nil {
			return nil
		}

		// Check if error is retryable (connection issue or not leader)
		if !isRetryableControlPlaneError(err) {
			slog.Debug("Request failed with non-retryable error", "error", err)
			return err
		}

		slog.Debug("Request failed with retryable error, will try other servers", "error", err)
	}

	// Try all control plane servers
	var lastErr error
	for _, addr := range n.controlPlaneAddrs {
		slog.Debug("Trying control plane server", "address", addr)

		client, conn, err := n.connectToControlPlaneServer(addr)
		if err != nil {
			slog.Debug("Failed to connect to control plane server", "address", addr, "error", err)
			lastErr = err
			continue
		}

		// Try the request
		err = requestFunc(client)
		if err == nil {
			// Update our current connection
			slog.Info("Successfully connected to control plane", "address", addr)

			n.controlPlaneMu.Lock()
			// Close old connection if exists
			if currentConnection != nil {
				currentConnection.conn.Close()
			}
			n.controlPlaneConnection = &ControlPlaneConnection{
				address: addr,
				client:  client,
				conn:    conn,
			}
			n.controlPlaneMu.Unlock()

			return nil
		}

		// Close connection since it didn't work
		conn.Close()

		// Check if error is retryable
		if !isRetryableControlPlaneError(err) {
			slog.Debug("Request failed with non-retryable error", "address", addr, "error", err)
			return err
		}

		slog.Debug("Request failed, will try next server", "address", addr, "error", err)
		lastErr = err
	}

	// All servers failed
	if lastErr != nil {
		slog.Error("All control plane servers failed", "error", lastErr)
		return fmt.Errorf("all control plane servers failed: %w", lastErr)
	}

	// No control plane servers available
	return fmt.Errorf("no control plane servers available")
}

// isRetryableControlPlaneError checks if an error indicates we should try another server
func isRetryableControlPlaneError(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		// Non-gRPC error, assume connection issue
		return true
	}

	code := st.Code()

	// Retry on connection errors or when server is not the leader
	return code == codes.Unavailable ||
		code == codes.FailedPrecondition ||
		code == codes.Unknown ||
		code == codes.DeadlineExceeded
}
