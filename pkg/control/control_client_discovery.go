package control

import (
	"context"
	"log/slog"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (cp *ControlPlane) GetClusterState(context context.Context, empty *emptypb.Empty) (*pb.GetClusterStateResponse, error) {
	if err := cp.ensureLeader(); err != nil {
		return nil, err
	}

	cp.mu.RLock()
	defer cp.mu.RUnlock()

	head := cp.getHead()
	tail := cp.getTail()

	if head == nil || tail == nil {
		slog.Info("GetClusterState: No nodes registered")

		return &pb.GetClusterStateResponse{
			Head: nil,
			Tail: nil,
		}, nil
	}

	slog.Info("GetClusterState",
		"head", head.Info,
		"tail", tail.Info,
	)

	return &pb.GetClusterStateResponse{
		Head: head.Info,
		Tail: tail.Info,
	}, nil
}
