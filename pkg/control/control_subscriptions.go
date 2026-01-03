package control

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type getSubscriptionNodeResult struct {
	nodeId string
	err    error
}

func (cp *ControlPlane) getSubscriptionNode() *getSubscriptionNodeResult {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if len(cp.nodes) == 0 {
		cp.logger.Info("GetSubscriptionNode: No nodes available")
		return &getSubscriptionNodeResult{"", status.Error(codes.Unavailable, "No nodes available")}
	}

	// Simple round-robin selection of nodes for subscription requests
	cp.lastControlIndex = (cp.lastControlIndex + 1) % uint64(len(cp.chain))

	nodeId := cp.chain[cp.lastControlIndex]
	return &getSubscriptionNodeResult{nodeId, nil}
}
