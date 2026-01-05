package shared

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ServerAddresses holds the addresses for HEAD and TAIL servers - this
// might be useful later when we add more complex functionality (subscriptions)
// ClientSet holds gRPC clients for different services
type ClientSet struct {
	// Connections
	ControlConn *grpc.ClientConn
	HeadConn    *grpc.ClientConn
	TailConn    *grpc.ClientConn

	// Clients
	Reads         pb.MessageBoardReadsClient
	Writes        pb.MessageBoardWritesClient
	Subscriptions pb.MessageBoardSubscriptionsClient

	ControlPlaneAddrs []string

	mu             sync.RWMutex
	controlPlaneMu sync.RWMutex
}

func (c *ClientSet) Close() {
	if c.ControlConn != nil {
		c.ControlConn.Close()
	}
	if c.HeadConn != nil {
		c.HeadConn.Close()
	}
	if c.TailConn != nil {
		c.TailConn.Close()
	}
}

func (c *ClientSet) resetClients() bool {
	// Get HEAD and TAIL addresses using the retry mechanism
	var headAddr, tailAddr string
	err := c.TryControlPlaneRequest(func(client pb.ClientDiscoveryClient) error {
		ctx, cancel := context.WithTimeout(context.Background(), Timeout)
		defer cancel()

		serverConns, err := client.GetClusterState(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}

		if serverConns.Head == nil || serverConns.Tail == nil {
			return status.Error(codes.NotFound, "no nodes available in the cluster")
		}

		headAddr = serverConns.Head.Address
		tailAddr = serverConns.Tail.Address
		return nil
	})

	if err != nil {
		return false
	}

	// Establish HEAD connection
	connHead, err := grpc.NewClient(
		headAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return false
	}

	// Establish TAIL connection
	connTail, err := grpc.NewClient(
		tailAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.TailConn = connTail
	c.HeadConn = connHead

	// Create gRPC clients for all services
	c.Writes = pb.NewMessageBoardWritesClient(connHead)
	c.Reads = pb.NewMessageBoardReadsClient(connTail)

	return true
}

func isServerUnavailableError(err error) bool {
	code := status.Code(err)

	switch code {
	case codes.Unavailable, codes.FailedPrecondition, codes.Internal:
		return true
	default:
		return false
	}
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

func RetryFetch[T any](ctx context.Context, c *ClientSet, fetchFunc func(context.Context) (T, error)) (T, error) {
	var zero T

	c.mu.RLock()
	result, err := fetchFunc(ctx)
	c.mu.RUnlock()

	if err == nil {
		return result, nil
	}

	// Is the error related to server unavailability?
	if !isServerUnavailableError(err) {
		return zero, err
	}

	// If so, try to reset clients
	if !c.resetClients() {
		return zero, fmt.Errorf("failed to reset clients: %w", err)
	}

	// If reset was successful, retry the fetch function
	c.mu.RLock()
	result, err = fetchFunc(ctx)
	c.mu.RUnlock()

	if err != nil {
		return zero, fmt.Errorf("retry failed after client reset: %w", err)
	}

	return result, nil
}

// connectToControlPlaneServer establishes a gRPC connection to a specific control plane server
func connectToControlPlaneServer(addr string) (pb.ClientDiscoveryClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	client := pb.NewClientDiscoveryClient(conn)
	return client, conn, nil
}

// TryControlPlaneRequest executes a request function against control plane servers
func (c *ClientSet) TryControlPlaneRequest(requestFunc func(pb.ClientDiscoveryClient) error) error {
	c.controlPlaneMu.RLock()
	currentConn := c.ControlConn
	c.controlPlaneMu.RUnlock()

	// Try current client first if we have one
	if currentConn != nil {
		currentClient := pb.NewClientDiscoveryClient(currentConn)
		err := requestFunc(currentClient)

		if err == nil {
			return nil
		}

		// Check if error is retryable (connection issue or not leader)
		if !isRetryableControlPlaneError(err) {
			return err
		}
	}

	// Try all control plane servers
	var lastErr error
	for _, addr := range c.ControlPlaneAddrs {
		client, conn, err := connectToControlPlaneServer(addr)
		if err != nil {
			lastErr = err
			continue
		}

		// Try the request
		err = requestFunc(client)
		if err == nil {
			c.controlPlaneMu.Lock()
			// Close old connection if exists
			if currentConn != nil {
				currentConn.Close()
			}
			c.ControlConn = conn
			c.controlPlaneMu.Unlock()

			return nil
		}

		// Close connection since it didn't work
		conn.Close()

		// Check if error is retryable
		if !isRetryableControlPlaneError(err) {
			return err
		}

		lastErr = err
	}

	// No control plane server could handle the request
	panic(fmt.Sprintf("Control plane is unreachable: %v", lastErr))
}
