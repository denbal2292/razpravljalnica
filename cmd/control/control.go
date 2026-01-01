package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/hashicorp/raft"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

/*
====================
FSM
====================
*/

type Command struct {
	Op    string
	Key   string
	Value string
}

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
	return fmt.Errorf("unknown op")
}

func (f *KVStoreFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c := make(map[string]string)
	for k, v := range f.data {
		c[k] = v
	}

	return &kvSnapshot{store: c}, nil
}

func (f *KVStoreFSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	return json.NewDecoder(rc).Decode(&f.data)
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

/*
====================
Raft setup
====================
*/

func createRaft(id, addr string, fsm raft.FSM, bootstrap bool) (*raft.Raft, error) {
	cfg := raft.DefaultConfig()
	cfg.LocalID = raft.ServerID(id)

	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	snapStore := raft.NewInmemSnapshotStore()

	// Persistent log & metadata stores
	// logStore, err := raftboltdb.NewBoltStore(fmt.Sprintf("raft-log-%s.bolt", id))
	// if err != nil {
	// 	return nil, err
	// }
	// stableStore, err := raftboltdb.NewBoltStore(fmt.Sprintf("raft-stable-%s.bolt", id))
	// if err != nil {
	// 	return nil, err
	// }

	// // Snapshot store on disk
	// snapStore, err := raft.NewFileSnapshotStore(".", 1, os.Stderr)

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	transport, err := raft.NewTCPTransport(
		addr,
		tcpAddr,
		3,
		10*time.Second,
		os.Stderr,
	)
	if err != nil {
		return nil, err
	}

	r, err := raft.NewRaft(cfg, fsm, logStore, stableStore, snapStore, transport)
	if err != nil {
		return nil, err
	}

	if bootstrap {
		f := r.BootstrapCluster(raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(id),
					Address: raft.ServerAddress(addr),
				},
			},
		})
		if err := f.Error(); err != nil {
			return nil, err
		}
	}

	return r, nil
}

/*
====================
gRPC RaftService
====================
*/

type RaftService struct {
	pb.UnimplementedRaftServiceServer
	r *raft.Raft
}

func NewRaftService(r *raft.Raft) *RaftService {
	return &RaftService{r: r}
}

func (s *RaftService) AddVoter(
	ctx context.Context,
	req *pb.NodeInfo,
) (*emptypb.Empty, error) {

	if s.r.State() != raft.Leader {
		return nil, fmt.Errorf("not leader")
	}

	cfg := s.r.GetConfiguration()
	if err := cfg.Error(); err != nil {
		return nil, err
	}

	for _, srv := range cfg.Configuration().Servers {
		if srv.ID == raft.ServerID(req.NodeId) {
			return &emptypb.Empty{}, nil
		}
	}

	f := s.r.AddVoter(
		raft.ServerID(req.NodeId),
		raft.ServerAddress(req.Address),
		0,
		0,
	)
	if err := f.Error(); err != nil {
		return nil, err
	}

	log.Printf("Added voter %s @ %s", req.NodeId, req.Address)
	return &emptypb.Empty{}, nil
}

/*
====================
Join client
====================
*/

func joinCluster(leaderAddr, id, raftAddr string) error {
	conn, err := grpc.NewClient(leaderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewRaftServiceClient(conn)

	_, err = client.AddVoter(
		context.Background(),
		&pb.NodeInfo{
			NodeId:  id,
			Address: raftAddr,
		},
	)
	return err
}

/*
====================
main
====================
*/

func main() {
	id := flag.String("id", "", "node id")
	addr := flag.String("addr", "", "raft bind address")
	bootstrap := flag.Bool("bootstrap", false, "bootstrap cluster")
	join := flag.String("join", "", "leader gRPC address")
	grpcAddr := flag.String("grpc", ":8000", "gRPC bind address")
	flag.Parse()

	if *id == "" || *addr == "" {
		log.Fatal("id and addr are required")
	}

	fsm := NewKVStoreFSM()

	r, err := createRaft(*id, *addr, fsm, *bootstrap)
	if err != nil {
		log.Fatalf("raft init failed: %v", err)
	}

	// gRPC server (runs on all nodes; leader-only logic inside)
	lis, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		log.Fatalf("grpc listen failed: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRaftServiceServer(grpcServer, NewRaftService(r))

	go func() {
		log.Printf("gRPC listening on %s", *grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("grpc serve failed: %v", err)
		}
	}()

	// Join loop
	if *join != "" {
		go func() {
			for {
				time.Sleep(2 * time.Second)
				if err := joinCluster(*join, *id, *addr); err != nil {
					log.Printf("join failed, retrying: %v", err)
					continue
				}
				log.Println("successfully joined raft cluster")
				return
			}
		}()
	}

	select {}
}
