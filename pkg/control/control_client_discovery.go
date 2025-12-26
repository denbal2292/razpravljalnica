package control

import (
	"context"
	"log"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (cp *ControlPlane) GetClusterState(context context.Context, empty *emptypb.Empty) (*pb.GetClusterStateResponse, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if len(cp.nodes) == 0 {
		log.Println("GetClusterState: No nodes registered")
		return &pb.GetClusterStateResponse{
			Head: nil,
			Tail: nil,
		}, nil
	}

	head := cp.nodes[0].Info
	tail := cp.nodes[len(cp.nodes)-1].Info

	log.Printf("GetClusterState: Head=%v, Tail=%v\n", head, tail)

	return &pb.GetClusterStateResponse{
		Head: head,
		Tail: tail,
	}, nil
}
