package control

import (
	"fmt"
	"io"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// Applies a single Raft log entry to the FSM
// The only place where the ControlPlane state is modified
// Called on the leader and then replicated to followers (via Raft)
func (cp *ControlPlane) Apply(l *raft.Log) interface{} {
	var cmd pb.RaftCommand
	if err := proto.Unmarshal(l.Data, &cmd); err != nil {
		return err
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	switch cmd.Op {
	case pb.RaftCommandType_OP_REGISTER:
		return cp.registerNode(cmd.Node, cmd.CreatedAt.AsTime())
	case pb.RaftCommandType_OP_UNREGISTER:
		return cp.unregisterNode(cmd.Node)
	case pb.RaftCommandType_OP_HEARTBEAT:
		return cp.heartbeat(cmd.Node)
	case pb.RaftCommandType_OP_SUBSCRIBE:
		return cp.getSubscriptionNode()
	default:
		panic(fmt.Errorf("unknown command op: %v", cmd.Op))
	}
}

// Snapshot returns a snapshot of the current FSM state
// Used for Raft snapshots for quicker recovery of followers
func (cp *ControlPlane) Snapshot() (raft.FSMSnapshot, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// TODO: Add last heartbeat times?
	nodes := make(map[string]*pb.NodeInfo)
	for id, node := range cp.nodes {
		nodes[id] = node.Info
	}

	return &fsmSnapshot{
		state: &pb.RaftSnapshot{
			Nodes:            nodes,
			Chain:            cp.chain,
			LastControlIndex: cp.lastControlIndex,
		},
	}, nil
}

// Restore restores the FSM from a snapshot
// Used for Raft snapshots for quicker recovery of followers
func (cp *ControlPlane) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	b, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	var s pb.RaftSnapshot
	if err := proto.Unmarshal(b, &s); err != nil {
		return err
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.chain = s.Chain
	cp.lastControlIndex = s.LastControlIndex
	cp.nodes = make(map[string]*NodeInfo)

	for id, info := range s.Nodes {
		client, _ := grpc.NewClient(info.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		cp.nodes[id] = &NodeInfo{
			Info:          info,
			Client:        pb.NewNodeUpdateClient(client),
			LastHeartbeat: time.Now(),
		}
	}
	return nil
}

type fsmSnapshot struct {
	state *pb.RaftSnapshot
}

// Used for saving Raft snapshots
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		b, err := proto.Marshal(s.state)
		if err != nil {
			return err
		}
		if _, err := sink.Write(b); err != nil {
			return err
		}
		return sink.Close()
	}()
	if err != nil {
		sink.Cancel()
	}
	return err
}

// Called after snapshot is no longer needed
// (unused in this implementation)
func (s *fsmSnapshot) Release() {}
