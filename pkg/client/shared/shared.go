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

	mu sync.RWMutex
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
	c.mu.RLock()
	controlConn := c.ControlConn
	c.mu.RUnlock()

	// Get HEAD and TAIL addresses
	headAddr, tailAddr, err := getHeadAndTailAddresses(controlConn)

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

// This might be useful later as a helper function upon server failure
func getHeadAndTailAddresses(controlPlaneConn *grpc.ClientConn) (headAddr, tailAddr string, err error) {
	controlPlaneClient := pb.NewClientDiscoveryClient(controlPlaneConn)

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	// Get addresses of HEAD and TAIL servers from the control plane
	serverConns, err := controlPlaneClient.GetClusterState(
		ctx, &emptypb.Empty{},
	)

	if err != nil {
		return "", "", err
	}

	// Get the HEAD and TAIL addresses
	headAddr = serverConns.Head.Address
	tailAddr = serverConns.Tail.Address

	return headAddr, tailAddr, nil
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
