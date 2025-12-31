package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// Command represents a set operation for the FSM
type Command struct {
	Op    string
	Key   string
	Value string
}

// KVStoreFSM is a simple FSM for a key-value store
type KVStoreFSM struct {
	mu   sync.Mutex
	data map[string]string
}

func NewKVStoreFSM() *KVStoreFSM {
	return &KVStoreFSM{data: make(map[string]string)}
}

func (f *KVStoreFSM) Apply(log *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if cmd.Op == "set" {
		f.data[cmd.Key] = cmd.Value
		return nil
	}

	return fmt.Errorf("unknown op: %s", cmd.Op)
}

func (f *KVStoreFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	clone := make(map[string]string)
	for k, v := range f.data {
		clone[k] = v
	}
	return &kvSnapshot{store: clone}, nil
}

func (f *KVStoreFSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	dec := json.NewDecoder(rc)
	data := make(map[string]string)
	if err := dec.Decode(&data); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data = data
	return nil
}

type kvSnapshot struct {
	store map[string]string
}

func (s *kvSnapshot) Persist(sink raft.SnapshotSink) error {
	b, err := json.Marshal(s.store)
	if err != nil {
		sink.Cancel()
		return err
	}
	if _, err := sink.Write(b); err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *kvSnapshot) Release() {}

// createRaftNode creates a raft node with the given id and bind address
func createRaftNode(id, bindAddr string, fsm raft.FSM, peers []raft.Server) (*raft.Raft, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(id)

	// Use in-memory log and stable store
	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	snapStore := raft.NewInmemSnapshotStore()

	// TCP transport
	addr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return nil, err
	}
	transport, err := raft.NewTCPTransport(bindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, err
	}

	r, err := raft.NewRaft(config, fsm, logStore, stableStore, snapStore, transport)
	if err != nil {
		return nil, err
	}

	// Bootstrap cluster if this is the first node
	if id == "node1" {
		configuration := raft.Configuration{Servers: peers}
		r.BootstrapCluster(configuration)
	}

	return r, nil
}

func main() {
	// Node addresses
	nodes := []struct {
		id   string
		addr string
	}{
		{"node1", "127.0.0.1:9001"},
		{"node2", "127.0.0.1:9002"},
		{"node3", "127.0.0.1:9003"},
	}

	// Peers for bootstrap
	peers := []raft.Server{
		{ID: "node1", Address: "127.0.0.1:9001"},
		{ID: "node2", Address: "127.0.0.1:9002"},
		{ID: "node3", Address: "127.0.0.1:9003"},
	}

	// Create FSMs and Raft nodes
	fsms := make([]*KVStoreFSM, 3)
	rafts := make([]*raft.Raft, 3)
	for i, n := range nodes {
		fsms[i] = NewKVStoreFSM()

		r, err := createRaftNode(n.id, n.addr, fsms[i], peers)

		if err != nil {
			log.Fatalf("failed to create raft node %s: %v", n.id, err)
		}

		rafts[i] = r
	}

	// Wait for leader election
	time.Sleep(2 * time.Second)

	// Find leader
	var leader *raft.Raft
	for i, r := range rafts {
		if r.State() == raft.Leader {
			leader = r
			fmt.Printf("Leader is %s\n", nodes[i].id)
			break
		}
	}
	if leader == nil {
		log.Fatal("No leader elected")
	}

	// Propose a command (set foo=bar)
	cmd := Command{Op: "set", Key: "foo", Value: "bar"}
	b, _ := json.Marshal(cmd)
	f := leader.Apply(b, 2*time.Second)
	if err := f.Error(); err != nil {
		log.Fatalf("apply error: %v", err)
	}
	fmt.Println("Set foo=bar via Raft")

	// Wait for replication
	time.Sleep(1 * time.Second)

	// Print state from all FSMs
	for i, fsm := range fsms {
		fsm.mu.Lock()
		v := fsm.data["foo"]
		fsm.mu.Unlock()
		fmt.Printf("Node %s: foo=%q\n", nodes[i].id, v)
	}
}
