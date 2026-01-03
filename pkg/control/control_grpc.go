package control

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

func serializeRaftCommand(cmd *pb.RaftCommand) []byte {
	data, err := proto.Marshal(cmd)
	if err != nil {
		panic("Failed to serialize Raft command: " + err.Error())
	}

	return data
}

func applyRaftCommand[T any](cp *ControlPlane, cmd *pb.RaftCommand) (T, error) {
	var zero T
	data := serializeRaftCommand(cmd)
	resultCh := cp.raft.Apply(data, cp.raftTimeout)

	if err := resultCh.Error(); err != nil {
		return zero, err
	}

	result := resultCh.Response()
	if resultErr, ok := result.(error); ok {
		return zero, resultErr
	}

	typedResult, ok := result.(T)
	if !ok {
		panic(fmt.Errorf("failed to cast result to type %T, got %T", zero, result))
	}

	return typedResult, nil
}

// TODO: Reject requests if not leader
func (cp *ControlPlane) RegisterNode(ctx context.Context, nodeInfo *pb.NodeInfo) (*pb.NeighborsInfo, error) {
	// Build the Raft command which will modify the cluster state
	raftCommand := &pb.RaftCommand{
		Op:        pb.RaftCommandType_OP_REGISTER,
		Node:      nodeInfo,
		CreatedAt: timestamppb.New(time.Now()),
	}

	// Apply the command via Raft
	result, raftErr := applyRaftCommand[*registerNodeResult](cp, raftCommand)
	if raftErr != nil {
		return nil, raftErr
	}

	predecessor := cp.getNode(result.predecessorId)

	return &pb.NeighborsInfo{
		Predecessor: predecessor.Info,
		Successor:   nil, // New node is always TAIL, so no successor
	}, result.err
}

func (cp *ControlPlane) UnregisterNode(ctx context.Context, nodeInfo *pb.NodeInfo) (*emptypb.Empty, error) {
	raftCommand := &pb.RaftCommand{
		Op:        pb.RaftCommandType_OP_UNREGISTER,
		Node:      nodeInfo,
		CreatedAt: timestamppb.New(time.Now()),
	}

	result, raftErr := applyRaftCommand[*registerNodeResult](cp, raftCommand)
	if raftErr != nil {
		return nil, raftErr
	}

	pred, succ, isValid := cp.getNodes(result.predecessorId, result.successorId)

	if isValid {
		go cp.reconnectNeighbors(pred, succ)
	}

	return &emptypb.Empty{}, nil
}

func (cp *ControlPlane) Heartbeat(context context.Context, nodeInfo *pb.NodeInfo) (*emptypb.Empty, error) {
	raftCommand := &pb.RaftCommand{
		Op:        pb.RaftCommandType_OP_HEARTBEAT,
		Node:      nodeInfo,
		CreatedAt: timestamppb.New(time.Now()),
	}

	heartbeatErr, raftErr := applyRaftCommand[error](cp, raftCommand)

	if raftErr != nil {
		return &emptypb.Empty{}, raftErr
	}

	return &emptypb.Empty{}, heartbeatErr
}

func (cp *ControlPlane) GetSubscriptionNode(ctx context.Context, req *pb.SubscriptionNodeRequest) (*pb.SubscriptionNodeResponse, error) {
	raftCommand := &pb.RaftCommand{
		Op:        pb.RaftCommandType_OP_SUBSCRIBE,
		CreatedAt: timestamppb.New(time.Now()),
	}

	result, raftErr := applyRaftCommand[*getSubscriptionNodeResult](cp, raftCommand)
	if raftErr != nil {
		return nil, raftErr
	}

	if result.err != nil {
		return nil, result.err
	}

	selectedNodeId := result.nodeId
	selectedNode := cp.nodes[selectedNodeId]

	addSub, err := selectedNode.Client.AddSubscriptionRequest(ctx, req)
	if err != nil {
		cp.logNodeError(selectedNode, err, "Failed to add subscription request to node")
		return nil, err
	}

	cp.logNodeInfo(selectedNode, "GetSubscriptionNode: Subscription node selected")

	return &pb.SubscriptionNodeResponse{Node: selectedNode.Info, SubscribeToken: addSub.SubscribeToken}, nil
}
