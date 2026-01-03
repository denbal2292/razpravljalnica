package main

import (
	"flag"
	"log"
	"net"
	"os"
	"time"

	"github.com/denbal2292/razpravljalnica/pkg/control"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc"
)

// Static cluster configuration
var clusterConfig = []struct {
	id       string
	raftAddr string
	grpcAddr string
}{
	{"node0", "127.0.0.1:7000", "127.0.0.1:50051"},
	{"node1", "127.0.0.1:7001", "127.0.0.1:50052"},
	{"node2", "127.0.0.1:7002", "127.0.0.1:50053"},
}

func createRaft(nodeIdx int, cp *control.ControlPlane) (*raft.Raft, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(clusterConfig[nodeIdx].id)

	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	snapStore := raft.NewInmemSnapshotStore()

	tcpAddr, err := net.ResolveTCPAddr("tcp", clusterConfig[nodeIdx].raftAddr)
	if err != nil {
		return nil, err
	}

	transport, err := raft.NewTCPTransport(
		clusterConfig[nodeIdx].raftAddr,
		tcpAddr,
		3,
		10*time.Second,
		os.Stderr,
	)
	if err != nil {
		return nil, err
	}

	r, err := raft.NewRaft(config, cp, logStore, stableStore, snapStore, transport)
	if err != nil {
		return nil, err
	}

	// Bootstrap cluster with all three nodes (only on node0)
	if nodeIdx == 0 {
		servers := make([]raft.Server, len(clusterConfig))
		for i, cfg := range clusterConfig {
			servers[i] = raft.Server{
				ID:      raft.ServerID(cfg.id),
				Address: raft.ServerAddress(cfg.raftAddr),
			}
		}

		f := r.BootstrapCluster(raft.Configuration{Servers: servers})
		if err := f.Error(); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func main() {
	nodeIdx := flag.Int("node", -1, "node index (0, 1, or 2)")
	flag.Parse()

	if *nodeIdx < 0 || *nodeIdx >= len(clusterConfig) {
		log.Fatal("node index must be 0, 1, or 2")
	}

	cp := control.NewControlPlane()
	r, err := createRaft(*nodeIdx, cp)
	if err != nil {
		log.Fatalf("raft init failed: %v", err)
	}

	cp.SetRaft(r)

	// Start gRPC server
	lis, err := net.Listen("tcp", clusterConfig[*nodeIdx].grpcAddr)
	if err != nil {
		log.Fatalf("grpc listen failed: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterClientDiscoveryServer(grpcServer, cp)
	pb.RegisterControlPlaneServer(grpcServer, cp)

	log.Printf("Node %s: gRPC listening on %s, Raft on %s",
		clusterConfig[*nodeIdx].id,
		clusterConfig[*nodeIdx].grpcAddr,
		clusterConfig[*nodeIdx].raftAddr)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve failed: %v", err)
	}

}
