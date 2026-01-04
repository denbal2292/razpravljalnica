package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) createClientConnection(address string) (*NodeConnection, error) {
	// Create gRPC connection to the given address
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		n.logger.Error("Failed to create gRPC client connection", "address", address, "error", err)
		return nil, err
	}

	return &NodeConnection{
		client: pb.NewChainReplicationClient(conn),
		conn:   conn,
	}, nil
}

// gRPC method to set the predecessor node
func (n *Node) SetPredecessor(ctx context.Context, predMsg *pb.NodeInfoMessage) (*emptypb.Empty, error) {
	pred := predMsg.Node

	n.ackGoroutineMu.Lock()
	defer n.ackGoroutineMu.Unlock()

	n.ackCancelFunc() // Cancel any existing ACK processor goroutine
	n.ackWg.Wait()    // Wait for it to finish

	n.setPredecessor(pred)

	n.logger.Info("SetPredecessor called", "node_info", pred)

	// Restart the ACK processor goroutine after updating predecessor
	n.startAckProcessorGoroutine()

	return &emptypb.Empty{}, nil
}

// gRPC method to set the successor node
func (n *Node) SetSuccessor(ctx context.Context, succMsg *pb.NodeInfoMessage) (*emptypb.Empty, error) {
	succ := succMsg.Node
	n.logger.Info("SetSuccessor called", "node_info", succ)

	go func() {
		n.syncMu.Lock()
		defer n.syncMu.Unlock()

		n.sendCancelFunc() // Cancel any existing sending goroutine
		n.sendWg.Wait()    // Wait for it to finish

		// Update the successor connection
		n.setSuccessor(succ)

		// Start syncing with the new successor if not nil
		if succ == nil {
			n.logger.Info("This node is TAIL, applying all unacknowledged events")
			n.applyAllUnacknowledgedEvents()
		} else {
			n.logger.Info("Starting sync with successor after SetSuccessor")
			n.syncWithSuccessor()
		}

		// Restart the event replication goroutine after sync
		n.startEventReplicationGoroutine()
	}()

	return &emptypb.Empty{}, nil
}
