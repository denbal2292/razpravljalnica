package server

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (n *Node) createClientConnection(address string) pb.ChainReplicationClient {
	// Create gRPC connection to the given address
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		n.logger.Error("Failed to create gRPC client connection", "address", address, "error", err)
		return nil
	}

	return pb.NewChainReplicationClient(conn)
}

func (n *Node) SetPredecessor(context context.Context, pred *pb.NodeInfo) (*emptypb.Empty, error) {
	n.logger.Info("SetPredecessor called - data syncing not implemented", "node_info", pred)

	n.predecessor = &NodeConnection{
		Client: n.createClientConnection(pred.Address),
	}
	return &emptypb.Empty{}, nil
}

func (n *Node) SetSuccessor(context context.Context, succ *pb.NodeInfo) (*emptypb.Empty, error) {
	n.logger.Info("SetSuccessor called - data syncing not implemented", "node_info", succ)

	n.successor = &NodeConnection{
		Client: n.createClientConnection(succ.Address),
	}
	return &emptypb.Empty{}, nil
}

