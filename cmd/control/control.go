package main

import (
	"flag"
	"log"
	"log/slog"
	"net"
	"time"

	"github.com/denbal2292/razpravljalnica/pkg/control"
	"github.com/denbal2292/razpravljalnica/pkg/control/gui"
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

func createRaft(nodeIdx int, cp *control.ControlPlane, raftLogger *slog.Logger) (*raft.Raft, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(clusterConfig[nodeIdx].id)

	// Use the raftLogger adapter for Raft logs
	config.Logger = gui.NewSlogAdapter(raftLogger, "raft")

	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	snapStore := raft.NewInmemSnapshotStore()

	tcpAddr, err := net.ResolveTCPAddr("tcp", clusterConfig[nodeIdx].raftAddr)
	if err != nil {
		return nil, err
	}

	// Create a custom writer for transport logs
	transportWriter := gui.NewSlogWriter(raftLogger)

	transport, err := raft.NewTCPTransport(
		clusterConfig[nodeIdx].raftAddr,
		tcpAddr,
		3,
		10*time.Second,
		transportWriter,
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
	interfaceType := flag.String("type", "terminal", "Interface type: terminal or gui")
	flag.Parse()

	if *nodeIdx < 0 || *nodeIdx >= len(clusterConfig) {
		log.Fatal("node index must be 0, 1, or 2")
	}

	// Initialize GUI or console mode
	logger, raftLogger, stats, cleanup := gui.StartWithFallback(*interfaceType == "gui")
	// Stop the app on exit
	defer cleanup()

	// Set default logger
	slog.SetDefault(logger)

	// Create control plane instance
	cp := control.NewControlPlane()

	// Set node information for stats display
	cp.SetNodeInfo(
		clusterConfig[*nodeIdx].id,
		clusterConfig[*nodeIdx].grpcAddr,
		clusterConfig[*nodeIdx].raftAddr,
	)

	// Set the control plane as the stats provider
	stats.SetProvider(cp)

	r, err := createRaft(*nodeIdx, cp, raftLogger)
	if err != nil {
		logger.Error("raft init failed", "error", err)
		log.Fatal(err)
	}

	cp.SetRaft(r)

	// Start gRPC server
	lis, err := net.Listen("tcp", clusterConfig[*nodeIdx].grpcAddr)
	if err != nil {
		logger.Error("grpc listen failed", "error", err)
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterClientDiscoveryServer(grpcServer, cp)
	pb.RegisterControlPlaneServer(grpcServer, cp)

	logger.Info("Control plane node started",
		"node_id", clusterConfig[*nodeIdx].id,
		"grpc_addr", clusterConfig[*nodeIdx].grpcAddr,
		"raft_addr", clusterConfig[*nodeIdx].raftAddr,
		"interface_type", *interfaceType)

	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("grpc serve failed", "error", err)
		log.Fatal(err)
	}
}
