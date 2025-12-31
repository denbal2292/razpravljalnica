package control

import (
	"log/slog"
	"os"
	"sync"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/lmittmann/tint"
)

type NodeInfo struct {
	Info          *pb.NodeInfo
	Client        pb.NodeUpdateClient
	LastHeartbeat time.Time
}

type ControlPlane struct {
	pb.UnimplementedClientDiscoveryServer // For clients connecting to find HEAD and TAIL nodes
	pb.UnimplementedControlPlaneServer    // For nodes connecting to report heartbeats

	mu                sync.RWMutex
	nodes             []*NodeInfo // nodes in order: [HEAD, ..., TAIL] (easier to get neighbors)
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration

	lastControlIndex uint64

	logger *slog.Logger
}

func NewControlPlane() *ControlPlane {
	cb := &ControlPlane{
		nodes:             make([]*NodeInfo, 0),
		heartbeatInterval: 5 * time.Second,
		heartbeatTimeout:  7 * time.Second,
		logger: slog.New(tint.NewHandler(
			os.Stdout,
			&tint.Options{
				Level: slog.LevelInfo,
				// GO's default reference time
				TimeFormat: "02-01-2006 15:04:05",
			},
		)),
	}

	// Start monitoring heartbeats
	go cb.monitorHeartbeats()

	return cb
}
