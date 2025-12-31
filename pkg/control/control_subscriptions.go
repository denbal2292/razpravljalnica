package control

import (
	"context"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (cp *ControlPlane) GetSubscriptionNode(ctx context.Context, req *pb.SubscriptionNodeRequest) (*pb.SubscriptionNodeResponse, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if len(cp.nodes) == 0 {
		cp.logger.Info("GetSubscriptionNode: No nodes available")
		return nil, status.Error(codes.Internal, "No nodes available")
	}

	// Simple round-robin selection of nodes for subscription
	cp.lastControlIndex = (cp.lastControlIndex + 1) % uint64(len(cp.nodes))
	selectedNode := cp.nodes[cp.lastControlIndex]

	addSub, err := selectedNode.Client.AddSubscriptionRequest(ctx, req)
	if err != nil {
		cp.logNodeError(selectedNode, err, "Failed to add subscription request to node")
		return nil, err
	}

	cp.logNodeInfo(selectedNode, "GetSubscriptionNode: Subscription node selected")

	return &pb.SubscriptionNodeResponse{Node: selectedNode.Info, SubscribeToken: addSub.SubscribeToken}, nil
}
