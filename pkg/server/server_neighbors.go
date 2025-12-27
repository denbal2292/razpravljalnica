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
func (n *Node) SetPredecessor(context context.Context, pred *pb.NodeInfo) (*emptypb.Empty, error) {
	n.logger.Info("SetPredecessor called - data syncing not implemented", "node_info", pred)
	n.setPredecessor(pred)

	return &emptypb.Empty{}, nil
}

// gRPC method to set the successor node
func (n *Node) SetSuccessor(context context.Context, succ *pb.NodeInfo) (*emptypb.Empty, error) {
	n.logger.Info("SetSuccessor called - data syncing not implemented", "node_info", succ)
	n.setSuccessor(succ)

	return &emptypb.Empty{}, nil
}
