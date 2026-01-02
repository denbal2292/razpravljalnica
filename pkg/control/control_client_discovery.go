package control

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (cp *ControlPlane) GetClusterState(context context.Context, empty *emptypb.Empty) (*pb.GetClusterStateResponse, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	head := cp.getHead()
	tail := cp.getTail()

	if head == nil || tail == nil {
		cp.logger.Info("GetClusterState: No nodes registered")
	} else {
		cp.logger.Info("GetClusterState",
			"head", head.Info,
			"tail", tail.Info,
		)
	}

	return &pb.GetClusterStateResponse{
		Head: head.Info,
		Tail: tail.Info,
	}, nil
}
