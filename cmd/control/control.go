package main

import (
	"flag"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/denbal2292/razpravljalnica/pkg/control"
	"github.com/denbal2292/razpravljalnica/pkg/control/gui"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc"
)

// slogWriter wraps slog.Logger to implement io.Writer for Raft transport.
type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	w.logger.Info(string(p))
	return len(p), nil
}

// slogAdapter adapts slog.Logger to hclog.Logger interface for Raft.
type slogAdapter struct {
	logger *slog.Logger
	name   string
}

func newSlogAdapter(logger *slog.Logger, name string) hclog.Logger {
	return &slogAdapter{logger: logger, name: name}
}

func (a *slogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Trace, hclog.Debug:
		a.logger.Debug(msg, args...)
	case hclog.Info:
		a.logger.Info(msg, args...)
	case hclog.Warn:
		a.logger.Warn(msg, args...)
	case hclog.Error:
		a.logger.Error(msg, args...)
	}
}

func (a *slogAdapter) Trace(msg string, args ...interface{}) { a.logger.Debug(msg, args...) }
func (a *slogAdapter) Debug(msg string, args ...interface{}) { a.logger.Debug(msg, args...) }
func (a *slogAdapter) Info(msg string, args ...interface{})  { a.logger.Info(msg, args...) }
func (a *slogAdapter) Warn(msg string, args ...interface{})  { a.logger.Warn(msg, args...) }
func (a *slogAdapter) Error(msg string, args ...interface{}) { a.logger.Error(msg, args...) }

func (a *slogAdapter) IsTrace() bool { return true }
func (a *slogAdapter) IsDebug() bool { return true }
func (a *slogAdapter) IsInfo() bool  { return true }
func (a *slogAdapter) IsWarn() bool  { return true }
func (a *slogAdapter) IsError() bool { return true }

func (a *slogAdapter) ImpliedArgs() []interface{}            { return nil }
func (a *slogAdapter) With(args ...interface{}) hclog.Logger { return a }
func (a *slogAdapter) Name() string                          { return a.name }
func (a *slogAdapter) Named(name string) hclog.Logger        { return newSlogAdapter(a.logger, name) }
func (a *slogAdapter) ResetNamed(name string) hclog.Logger   { return newSlogAdapter(a.logger, name) }
func (a *slogAdapter) SetLevel(level hclog.Level)            {}
func (a *slogAdapter) GetLevel() hclog.Level                 { return hclog.Debug }
func (a *slogAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}
func (a *slogAdapter) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return os.Stderr
}

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
	config.Logger = newSlogAdapter(raftLogger, "raft")

	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	snapStore := raft.NewInmemSnapshotStore()

	tcpAddr, err := net.ResolveTCPAddr("tcp", clusterConfig[nodeIdx].raftAddr)
	if err != nil {
		return nil, err
	}

	// Create a custom writer for transport logs
	transportWriter := &slogWriter{logger: raftLogger}

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
