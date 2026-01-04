package server

import (
	"fmt"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// connectToControlPlaneServer establishes a gRPC connection to a specific control plane server
func (n *Node) connectToControlPlaneServer(addr string) (pb.ControlPlaneClient, *grpc.ClientConn, error) {
	n.logger.Debug("Attempting to connect to control plane server", "address", addr)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		n.logger.Debug("Failed to create gRPC client", "address", addr, "error", err)
		return nil, nil, err
	}

	client := pb.NewControlPlaneClient(conn)
	n.logger.Debug("Successfully connected to control plane server", "address", addr)

	return client, conn, nil
}

// tryControlPlaneRequest executes a request function against control plane servers
// It tries the current server first, then all others if it fails due to connection
// issues or the server not being the leader
func (n *Node) tryControlPlaneRequest(requestFunc func(pb.ControlPlaneClient) error) error {
	n.controlPlaneMu.RLock()
	currentClient := n.controlPlane
	currentConn := n.controlPlaneConn
	n.controlPlaneMu.RUnlock()

	// Try current client first if we have one
	if currentClient != nil {
		n.logger.Debug("Trying request with current control plane client")
		err := requestFunc(currentClient)

		if err == nil {
			n.logger.Debug("Request succeeded with current control plane client")
			return nil
		}

		// Check if error is retryable (connection issue or not leader)
		if !isRetryableControlPlaneError(err) {
			n.logger.Debug("Request failed with non-retryable error", "error", err)
			return err
		}

		n.logger.Debug("Request failed with retryable error, will try other servers", "error", err)
	}

	// Try all control plane servers
	var lastErr error
	for _, addr := range n.controlPlaneAddrs {
		n.logger.Debug("Trying control plane server", "address", addr)

		client, conn, err := n.connectToControlPlaneServer(addr)
		if err != nil {
			n.logger.Debug("Failed to connect to control plane server", "address", addr, "error", err)
			lastErr = err
			continue
		}

		// Try the request
		err = requestFunc(client)
		if err == nil {
			// Success! Update our current client
			n.logger.Info("Successfully connected to control plane", "address", addr)

			n.controlPlaneMu.Lock()
			// Close old connection if exists
			if currentConn != nil {
				currentConn.Close()
			}
			n.controlPlane = client
			n.controlPlaneConn = conn
			n.controlPlaneMu.Unlock()

			return nil
		}

		// Close this connection since it didn't work
		conn.Close()

		// Check if error is retryable
		if !isRetryableControlPlaneError(err) {
			n.logger.Debug("Request failed with non-retryable error", "address", addr, "error", err)
			return err
		}

		n.logger.Debug("Request failed, will try next server", "address", addr, "error", err)
		lastErr = err
	}

	// All servers failed
	if lastErr != nil {
		n.logger.Error("All control plane servers failed", "error", lastErr)
		return fmt.Errorf("all control plane servers failed: %w", lastErr)
	}

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
